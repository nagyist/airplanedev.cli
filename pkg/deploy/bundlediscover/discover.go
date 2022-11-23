package bundlediscover

import (
	"context"
	"os"
	"path"
	"path/filepath"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

type Discoverer struct {
	TaskDiscoverers []discover.TaskDiscoverer
	ViewDiscoverers []discover.ViewDiscoverer
	Client          api.IAPIClient
	Logger          logger.Logger

	// EnvSlug is the slug of the environment to look for discovered tasks in.
	//
	// If a task is discovered, but doesn't exist in this environment, then the task
	// is treated as missing.
	EnvSlug string
}

// Bundle is a directory that may contain 1 or more tasks or views.
type Bundle struct {
	// RootPath is the absolute path to the root of the bundle.
	RootPath string
	// TargetPaths are file paths relative to the root that should be deployed.
	// Only entities that exist in these paths are deployed.
	// e.g. the root path may contain 5 individual tasks, but the user may only
	// want to deploy one of those tasks, specified by a single target path.
	TargetPaths  []string
	BuildContext build.BuildContext
}

// Discover recursively discovers Airplane bundles located within "paths".
func (d *Discoverer) Discover(ctx context.Context, paths ...string) ([]Bundle, error) {
	discoveredBundles, err := d.discoverHelper(ctx, paths...)
	if err != nil {
		return nil, err
	}

	// Dedupe discovered bundles.
	var dedupedBundles []Bundle
	for _, b := range discoveredBundles {
		var alreadyAdded bool
		for j, addedBundle := range dedupedBundles {
			if equal(addedBundle, b) {
				alreadyAdded = true
				// The bundle was already added. Add its target paths if they don't exist.
				for _, target := range b.TargetPaths {
					if err := updateBundleWithTarget(&addedBundle, path.Join(addedBundle.RootPath, target)); err != nil {
						return nil, err
					}
				}
				for k, v := range b.BuildContext.EnvVars {
					if addedBundle.BuildContext.EnvVars == nil {
						addedBundle.BuildContext.EnvVars = make(map[string]build.EnvVarValue)
					}
					addedBundle.BuildContext.EnvVars[k] = v
				}
				dedupedBundles[j] = addedBundle
			}
		}
		if !alreadyAdded {
			dedupedBundles = append(dedupedBundles, b)
		}
	}

	// If any of the bundles are children of other bundles, convert the child root to being == the parent root.
	// This ensures that we are creating as few bundles as possible.
	// In order to do this, we set the child root path to the parent root path and update the target paths to be relative
	// to the new (higher up) root.
	for _, b := range dedupedBundles {
		for i, b2 := range dedupedBundles {
			bIsParentOfB2, err := fsx.IsSubDirectory(b.RootPath, b2.RootPath)
			if err != nil {
				return nil, err
			}
			if bIsParentOfB2 {
				b2RelFromb1, err := filepath.Rel(b.RootPath, b2.RootPath)
				if err != nil {
					return nil, err
				}
				b2.RootPath = b.RootPath
				// Update the targets of b2 to be relative to the new root path.
				for ti, target := range b2.TargetPaths {
					b2.TargetPaths[ti] = path.Join(b2RelFromb1, target)
				}
				dedupedBundles[i] = b2
			}
		}
	}

	return dedupedBundles, nil
}

func (d *Discoverer) discoverHelper(ctx context.Context, paths ...string) ([]Bundle, error) {
	var bundles []Bundle

	for _, p := range paths {
		if discover.IgnoredDirectories[filepath.Base(p)] {
			continue
		}
		fileInfo, err := os.Stat(p)
		if err != nil {
			return nil, errors.Wrapf(err, "determining if %s is file or directory", p)
		}

		if fileInfo.IsDir() {
			// We found a directory. Recursively explore all of the files and directories in it.
			nestedFiles, err := os.ReadDir(p)
			if err != nil {
				return nil, errors.Wrapf(err, "reading directory %s", p)
			}
			var nestedPaths []string
			for _, nestedFile := range nestedFiles {
				nestedPaths = append(nestedPaths, path.Join(p, nestedFile.Name()))
			}
			nestedBundles, err := d.discoverHelper(ctx, nestedPaths...)
			if err != nil {
				return nil, err
			}
			for _, b := range nestedBundles {
				b.TargetPaths = nil
				if err := updateBundleWithTarget(&b, p); err != nil {
					return nil, err
				}
				bundles = append(bundles, b)
			}
		} else {
			// We found a file.
			for _, td := range d.TaskDiscoverers {
				bundlePath, buildContext, err := td.GetTaskRoot(ctx, p)
				if err != nil {
					return nil, err
				}
				if bundlePath == "" {
					// This file is not an Airplane task.
					continue
				}

				b := Bundle{
					RootPath:     bundlePath,
					BuildContext: buildContext,
				}
				if err := updateBundleWithTarget(&b, p); err != nil {
					return nil, err
				}
				bundles = append(bundles, b)
			}
			for _, td := range d.ViewDiscoverers {
				bundlePath, buildContext, err := td.GetViewRoot(ctx, p)
				if err != nil {
					return nil, err
				}
				if bundlePath == "" {
					// This file is not an Airplane view.
					continue
				}

				bundle := Bundle{
					RootPath:     bundlePath,
					BuildContext: buildContext,
				}
				bundles = append(bundles, bundle)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return bundles, nil
}

// updateBundleWithTarget adds a target path to a bundle, if it doesn't already exist.
// If the target is a parent of the bundle's root, the target is set to the root (".").
func updateBundleWithTarget(b *Bundle, target string) error {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	targetIsParentOfRoot, err := fsx.IsSubDirectory(absTarget, b.RootPath)
	if err != nil {
		return err
	}

	var relPath string
	if targetIsParentOfRoot {
		relPath = "."
	} else {
		relPath, err = filepath.Rel(b.RootPath, absTarget)
		if err != nil {
			return err
		}
	}

	if !slices.Contains(b.TargetPaths, relPath) {
		b.TargetPaths = append(b.TargetPaths, relPath)
	}
	return nil
}

func equal(b1, b2 Bundle) bool {
	// If the two bundles have the same env var with a different value, they are not equal.
	for k, v1 := range b1.BuildContext.EnvVars {
		v2, ok := b2.BuildContext.EnvVars[k]
		if ok {
			if v1.Config != nil {
				if v2.Config == nil || *v1.Config != *v2.Config {
					return false
				}
			} else if v1.Value != nil {
				if v2.Value == nil || *v1.Value != *v2.Value {
					return false
				}
			}
		}
	}
	return b1.RootPath == b2.RootPath &&
		b2.BuildContext.Type == b1.BuildContext.Type &&
		b2.BuildContext.Version == b1.BuildContext.Version &&
		b2.BuildContext.Base == b1.BuildContext.Base
}
