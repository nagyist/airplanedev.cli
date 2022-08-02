package utils

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/pkg/errors"
)

func CopyFile(srcPath string, dstDirectory string) error {
	src, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return errors.Wrapf(err, "reading src file %s", srcPath)
	}

	dstPath := filepath.Join(dstDirectory, filepath.Base(srcPath))
	if err := ioutil.WriteFile(dstPath, src, 0644); err != nil {
		return errors.Wrapf(err, "writing dst file %s", dstPath)
	}
	logger.Step("Copied %s", dstPath)
	return nil
}

func CopyDirectoryContents(srcDirectory string, dstDirectory string) error {
	return filepath.WalkDir(srcDirectory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == srcDirectory {
			return nil
		}

		rel, err := filepath.Rel(srcDirectory, path)
		if err != nil {
			return err
		}

		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dstDirectory, rel), 0755)
		}
		return CopyFile(path, filepath.Join(dstDirectory, filepath.Dir(rel)))
	})
}
