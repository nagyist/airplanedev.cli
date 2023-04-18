package initcmd

import (
	"fmt"
	"os"

	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/pkg/errors"
)

func createFolder(directory string) error {
	if _, err := os.Stat(directory); err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		// Directory doesn't exist, make it.
		if err := os.MkdirAll(directory, 0755); err != nil {
			return err
		}
	}
	return nil
}

func cwdIsHome() (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	return cwd == home, nil
}

const maxRandomAttempts = 10

var ErrTooManyAttempts = errors.New("exceeded maximum number of attempts to find a unique filename")

func trySuffix(filename string, addSuffix func(string, string) string, length int, charset string) (string, error) {
	if addSuffix == nil {
		addSuffix = func(s, suffix string) string {
			return fsx.AddFileSuffix(s, suffix)
		}
	}

	// Do we need the suffix?
	if !fsx.Exists(filename) {
		return filename, nil
	}

	// Try numeric suffixes: filename_[1-9].ext
	for i := 1; i < 10; i++ {
		suffix := fmt.Sprintf("%d", i)
		suffixedFilename := addSuffix(filename, suffix)
		if !fsx.Exists(suffixedFilename) {
			return suffixedFilename, nil
		}
	}

	// Try random suffixes: filename_[charset]{length}.ext
	for i := 0; i < maxRandomAttempts; i++ {
		suffix := utils.RandomString(length, charset)
		suffixedFilename := addSuffix(filename, suffix)
		if !fsx.Exists(suffixedFilename) {
			return suffixedFilename, nil
		}
	}
	return "", ErrTooManyAttempts
}
