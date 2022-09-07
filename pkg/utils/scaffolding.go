package utils

import (
	_ "embed"
	"io/ioutil"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

//go:embed scaffolding/.gitignore
var defaultGitignoreContents []byte

func CreateDefaultGitignoreFile(path string) error {
	if !fsx.Exists(path) {
		if err := ioutil.WriteFile(path, defaultGitignoreContents, 0644); err != nil {
			return errors.Wrap(err, "creating .gitignore")
		}
		logger.Step("Created .gitignore")
	}
	return nil
}
