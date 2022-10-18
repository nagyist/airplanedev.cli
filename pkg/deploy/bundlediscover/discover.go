package bundlediscover

import (
	"context"
	"os"
	"path"
	"path/filepath"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
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
	TargetPaths []string
	// TODO add build type and version
}

// Discover recursively discovers Airplane bundles located within "paths".
func (d *Discoverer) Discover(ctx context.Context, paths ...string) ([]Bundle, error) {
	bundleByPath := make(map[string]Bundle)

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
				existingBundle, ok := bundleByPath[b.RootPath]
				if !ok {
					existingBundle = Bundle{RootPath: b.RootPath}
				}
				bundle, err := updateBundleWithTarget(existingBundle, p)
				if err != nil {
					return nil, err
				}
				bundleByPath[bundle.RootPath] = bundle
			}
		} else {
			// We found a file.
			for _, td := range d.TaskDiscoverers {
				bundlePath, err := td.GetTaskRoot(ctx, p)
				if err != nil {
					return nil, err
				}
				if bundlePath == "" {
					// This file is not an Airplane task.
					continue
				}

				existingBundle, ok := bundleByPath[bundlePath]
				if !ok {
					existingBundle = Bundle{RootPath: bundlePath}
				}
				b, err := updateBundleWithTarget(existingBundle, p)
				if err != nil {
					return nil, err
				}
				bundleByPath[bundlePath] = b
			}
			for _, td := range d.ViewDiscoverers {
				bundlePath, err := td.GetViewRoot(ctx, p)
				if err != nil {
					return nil, err
				}
				if bundlePath == "" {
					// This file is not an Airplane view.
					continue
				}

				bundleByPath[bundlePath] = Bundle{RootPath: bundlePath}
			}
		}
	}

	var bundles []Bundle
	for _, b := range bundleByPath {
		bundles = append(bundles, b)
	}
	return bundles, nil
}

func updateBundleWithTarget(b Bundle, target string) (Bundle, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return Bundle{}, err
	}
	targetIsParentOfRoot, err := fsx.IsSubDirectory(absTarget, b.RootPath)
	if err != nil {
		return b, err
	}

	var relPath string
	if targetIsParentOfRoot {
		relPath = "."
	} else {
		relPath, err = filepath.Rel(b.RootPath, absTarget)
		if err != nil {
			return Bundle{}, err
		}
	}

	shouldAdd := true
	for _, existingTargetPath := range b.TargetPaths {
		if existingTargetPath == relPath {
			shouldAdd = false
			break
		}
		isSubDirectoryOfExistingPath, err := fsx.IsSubDirectory(existingTargetPath, relPath)
		if err != nil {
			return b, err
		}
		if isSubDirectoryOfExistingPath {
			shouldAdd = false
			break
		}
	}
	if shouldAdd {
		b.TargetPaths = append(b.TargetPaths, relPath)
	}
	return b, nil
}
