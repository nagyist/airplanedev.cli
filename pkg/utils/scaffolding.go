package utils

import (
	_ "embed"
	"os"

	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

//go:embed scaffolding/.gitignore
var defaultGitignoreContents []byte

func CreateDefaultGitignoreFile(path string) error {
	if ShouldCreateDefaultGitignoreFile(path) {
		if err := os.WriteFile(path, defaultGitignoreContents, 0644); err != nil {
			return errors.Wrap(err, "creating .gitignore")
		}
		logger.Step("Created .gitignore")
	}
	return nil
}

func ShouldCreateDefaultGitignoreFile(path string) bool {
	return !fsx.Exists(path)
}
