package hooks

import (
	"os"
	"path/filepath"

	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

const (
	preInstallScriptName  = "airplane_preinstall.sh"
	postInstallScriptName = "airplane_postinstall.sh"
)

type InstallHooks struct {
	// Paths are relative to the task's root
	PreInstallFilePath  string
	PostInstallFilePath string
}

// GetInstallHooks look for install scripts in every directory from the entrypoint's
// directory up to the root directory.
func GetInstallHooks(entrypoint string, root string) (InstallHooks, error) {
	find := func(filename string) (string, error) {
		dir, ok := fsx.FindUntil(
			// Entrypoint path is relative to the root so we need to combine them.
			filepath.Join(root, entrypoint),
			filepath.Dir(root),
			filename,
		)
		if !ok {
			return "", nil
		}
		abspath := filepath.Join(dir, filename)
		if _, err := os.ReadFile(abspath); err != nil {
			return "", errors.Wrapf(err, "install hooks: reading %s", abspath)
		}
		relpath, err := filepath.Rel(root, abspath)
		if err != nil {
			return "", errors.Wrapf(err, "install hooks: fetching relative path %s", abspath)
		}
		return relpath, nil
	}
	preInstallFilePath, err := find(preInstallScriptName)
	if err != nil {
		return InstallHooks{}, err
	}
	postInstallFilePath, err := find(postInstallScriptName)
	if err != nil {
		return InstallHooks{}, err
	}
	return InstallHooks{
		PreInstallFilePath:  preInstallFilePath,
		PostInstallFilePath: postInstallFilePath,
	}, nil
}
