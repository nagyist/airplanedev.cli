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
	BuildType    build.BuildType
	BuildVersion build.BuildTypeVersion
	BuildBase    build.BuildBase
}

// Discover recursively discovers Airplane bundles located within "paths".
func (d *Discoverer) Discover(ctx context.Context, paths ...string) ([]Bundle, error) {
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
			nestedBundles, err := d.Discover(ctx, nestedPaths...)
			if err != nil {
				return nil, err
			}
			for _, b := range nestedBundles {
				// Ignore nested target paths added in recursive calls. We only care about the top level.
				b.TargetPaths = nil
				bundles, err = addBundle(bundles, b, p)
				if err != nil {
					return nil, err
				}
			}
		} else {
			// We found a file.
			for _, td := range d.TaskDiscoverers {
				bundlePath, buildType, buildTypeVersion, buildBase, err := td.GetTaskRoot(ctx, p)
				if err != nil {
					return nil, err
				}
				if bundlePath == "" {
					// This file is not an Airplane task.
					continue
				}

				bundle := Bundle{
					RootPath:     bundlePath,
					BuildType:    buildType,
					BuildVersion: buildTypeVersion,
					BuildBase:    buildBase,
				}
				bundles, err = addBundle(bundles, bundle, p)
				if err != nil {
					return nil, err
				}
			}
			for _, td := range d.ViewDiscoverers {
				bundlePath, buildType, buildTypeVersion, buildBase, err := td.GetViewRoot(ctx, p)
				if err != nil {
					return nil, err
				}
				if bundlePath == "" {
					// This file is not an Airplane view.
					continue
				}

				bundle := Bundle{
					RootPath:     bundlePath,
					BuildType:    buildType,
					BuildVersion: buildTypeVersion,
					BuildBase:    buildBase,
				}
				bundles, err = addBundle(bundles, bundle, p)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return bundles, nil
}

func addBundle(bundles []Bundle, bundle Bundle, path string) ([]Bundle, error) {
	matchingBundle := -1
	for i, b := range bundles {
		match := b.RootPath == bundle.RootPath &&
			b.BuildType == bundle.BuildType &&
			b.BuildVersion == bundle.BuildVersion &&
			b.BuildBase == bundle.BuildBase
		if match {
			matchingBundle = i
			break
		}
	}

	if matchingBundle != -1 {
		b := bundles[matchingBundle]
		// Update the already existing bundle with the target path.
		if err := updateBundleWithTarget(&b, path); err != nil {
			return nil, err
		}
		bundles[matchingBundle] = b
		return bundles, nil
	}

	// Update the new bundle with the target path and append to the list of bundles.
	if err := updateBundleWithTarget(&bundle, path); err != nil {
		return nil, err
	}
	bundles = append(bundles, bundle)
	return bundles, nil
}

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
