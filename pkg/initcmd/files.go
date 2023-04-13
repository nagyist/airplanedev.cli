package initcmd

import (
	"os"
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
