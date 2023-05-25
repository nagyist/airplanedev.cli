package initcmd

import (
	"context"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/cli/prompts"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
)

type InitViewFromExampleRequest struct {
	Prompter    prompts.Prompter
	Logger      logger.Logger
	ExamplePath string
}

func InitViewFromExample(ctx context.Context, req InitViewFromExampleRequest) error {
	if err := utils.CopyFromGithubPath(req.Prompter, req.Logger, req.ExamplePath); err != nil {
		return err
	}
	viewDir := filepath.Base(req.ExamplePath)

	suggestNextViewSteps(suggestNextViewStepsRequest{
		logger:  req.Logger,
		viewDir: viewDir,
	})

	return nil
}
