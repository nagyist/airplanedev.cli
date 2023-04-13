package initcmd

import (
	"context"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/airplanedev/cli/pkg/utils"
)

type InitViewFromExampleRequest struct {
	Prompter    prompts.Prompter
	ExamplePath string
}

func InitViewFromExample(ctx context.Context, req InitViewFromExampleRequest) error {
	if err := utils.CopyFromGithubPath(req.ExamplePath, req.Prompter); err != nil {
		return err
	}
	viewDir := filepath.Base(req.ExamplePath)

	suggestNextViewSteps(suggestNextViewStepsRequest{
		viewDir: viewDir,
	})

	return nil
}
