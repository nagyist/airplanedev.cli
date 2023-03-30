package bundlediscover

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/api"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/utils/logger"
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
	BuildContext buildtypes.BuildContext
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
		var duplicate bool
		for j, addedBundle := range dedupedBundles {
			if equal(addedBundle, b) {
				// These two bundles are equal. Collapse them into one.
				duplicate = true
				for _, target := range b.TargetPaths {
					if err := updateBundleWithTarget(&addedBundle, path.Join(addedBundle.RootPath, target)); err != nil {
						return nil, err
					}
				}
				for k, v := range b.BuildContext.EnvVars {
					if addedBundle.BuildContext.EnvVars == nil {
						addedBundle.BuildContext.EnvVars = make(map[string]buildtypes.EnvVarValue)
					}
					addedBundle.BuildContext.EnvVars[k] = v
				}
				dedupedBundles[j] = addedBundle
			}
		}
		if !duplicate {
			dedupedBundles = append(dedupedBundles, b)
		}
	}

	// Dedupe targets in each bundle.
	for i := 0; i < len(dedupedBundles); i++ {
		for j := i + 1; j < len(dedupedBundles); j++ {
			b1 := dedupedBundles[i]
			b2 := dedupedBundles[j]
			if equalRootTypeVersionBase(b1, b2) {
				// These two bundles have the same root, type, version, and base, but clashing env vars.
				// We want to make sure that files only exist in one of the bundles.
				// There is no usecase for building the same file with different env vars.
				b1Targets := make(map[string]interface{})
				for _, target := range b1.TargetPaths {
					b1Targets[target] = struct{}{}
				}
				var b2Targets []string
				for _, target := range b2.TargetPaths {
					if _, ok := b1Targets[target]; !ok {
						b2Targets = append(b2Targets, target)
					}
				}
				b2.TargetPaths = b2Targets
				dedupedBundles[j] = b2
			}
		}
	}

	// Standardize target paths to use forward slashes.
	// On unix machines this will be a no-op, but on Windows machines this will
	// convert backslashes to forward slashes.
	for i, b := range dedupedBundles {
		for j, target := range b.TargetPaths {
			b.TargetPaths[j] = filepath.ToSlash(target)
		}
		dedupedBundles[i] = b
	}

	return dedupedBundles, nil
}

func (d *Discoverer) discoverHelper(ctx context.Context, paths ...string) ([]Bundle, error) {
	var bundles []Bundle

	for _, p := range paths {
		fileInfo, err := os.Stat(p)
		if err != nil {
			return nil, errors.Wrapf(err, "determining if %s is file or directory", p)
		}

		if !fileInfo.IsDir() {
			bundlesForFile, err := d.getBundlesForFile(ctx, p)
			if err != nil {
				return nil, err
			}
			bundles = append(bundles, bundlesForFile...)
		} else {
			err := filepath.WalkDir(p, func(path string, entry fs.DirEntry, err error) error {
				if discover.IgnoredDirectories[filepath.Base(path)] {
					return filepath.SkipDir
				}
				if err != nil {
					return err
				}

				if !entry.IsDir() {
					bundlesForFile, err := d.getBundlesForFile(ctx, path)
					if err != nil {
						return err
					}
					bundles = append(bundles, bundlesForFile...)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}

	return bundles, nil
}

func (d *Discoverer) getBundlesForFile(ctx context.Context, path string) ([]Bundle, error) {
	var bundles []Bundle
	for _, td := range d.TaskDiscoverers {
		bundlePath, buildContext, err := td.GetTaskRoot(ctx, path)
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
		if err := updateBundleWithTarget(&b, path); err != nil {
			return nil, err
		}
		bundles = append(bundles, b)
	}
	for _, td := range d.ViewDiscoverers {
		bundlePath, buildContext, err := td.GetViewRoot(ctx, path)
		if err != nil {
			return nil, err
		}
		if bundlePath == "" {
			// This file is not an Airplane view.
			continue
		}

		b := Bundle{
			RootPath:     bundlePath,
			BuildContext: buildContext,
		}
		if err := updateBundleWithTarget(&b, path); err != nil {
			return nil, err
		}
		bundles = append(bundles, b)
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

	relPath, err := filepath.Rel(b.RootPath, absTarget)
	if err != nil {
		return err
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
	return equalRootTypeVersionBase(b1, b2)
}

func equalRootTypeVersionBase(b1, b2 Bundle) bool {
	return b1.RootPath == b2.RootPath &&
		b2.BuildContext.Type == b1.BuildContext.Type &&
		b2.BuildContext.Version == b1.BuildContext.Version &&
		b2.BuildContext.Base == b1.BuildContext.Base
}
