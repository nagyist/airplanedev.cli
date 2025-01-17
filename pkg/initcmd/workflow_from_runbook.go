package initcmd

import (
	"context"
	"os"
	"path/filepath"

	api "github.com/airplanedev/cli/pkg/api/cliapi"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/airplanedev/cli/pkg/rb2wf"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

type InitWorkflowFromRunbookRequest struct {
	Client   api.APIClient
	Prompter prompts.Prompter
	Logger   logger.Logger

	File        string
	FromRunbook string

	Inline bool

	AssumeYes bool
	AssumeNo  bool
	EnvSlug   string
}

func InitWorkflowFromRunbook(ctx context.Context, req InitWorkflowFromRunbookRequest) error {
	if req.Logger == nil {
		req.Logger = logger.NewNoopLogger()
	}
	var entrypoint string
	var err error

	if req.AssumeYes && req.File != "" {
		entrypoint = req.File
	} else {
		entrypoint, err = promptForEntrypoint(promptForEntrypointRequest{
			prompter:          req.Prompter,
			slug:              req.FromRunbook,
			kind:              buildtypes.TaskKindNode,
			defaultEntrypoint: entrypoint,
			inline:            req.Inline,
		})
		if err != nil {
			return err
		}
	}

	entrypointDir := filepath.Dir(entrypoint)
	if err := os.MkdirAll(entrypointDir, 0744); err != nil {
		return errors.Wrap(err, "creating output directory")
	}

	// Create a definition that can be used to generate/update the package config.
	def := definitions.Definition{
		Node: &definitions.NodeDefinition{
			NodeVersion: "18",
			Base:        buildtypes.BuildBaseSlim,
		},
	}
	absEntrypoint, err := filepath.Abs(entrypoint)
	if err != nil {
		return errors.Wrap(err, "determining absolute entrypoint")
	}
	if err := def.SetAbsoluteEntrypoint(absEntrypoint); err != nil {
		return err
	}

	if err = runKindSpecificInstallation(ctx, runKindSpecificInstallationRequest{
		Prompter: req.Prompter,
		Logger:   req.Logger,
		Inline:   req.Inline,
		Kind:     buildtypes.TaskKindNode,
		Def:      def,
	}); err != nil {
		return err
	}

	converter := rb2wf.NewRunbookConverter(
		req.Client,
		entrypointDir,
		filepath.Base(entrypoint),
	)
	err = converter.Convert(ctx, req.FromRunbook, req.EnvSlug)
	if err != nil {
		return err
	}

	suggestNextTaskSteps(suggestNextTaskStepsRequest{
		logger:     req.Logger,
		entrypoint: entrypoint,
		kind:       buildtypes.TaskKindNode,
		isNew:      true,
	})

	return nil
}
