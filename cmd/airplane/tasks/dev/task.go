package dev

import (
	"context"

	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/pkg/errors"
)

type taskInfo struct {
	slug                string
	name                string
	kind                build.TaskKind
	kindOptions         build.KindOptions
	parameters          libapi.Parameters
	resourceAttachments map[string]string
}

func getTaskInfo(ctx context.Context, cfg taskDevConfig) (taskInfo, error) {
	switch definitions.GetTaskDefFormat(cfg.fileOrDir) {
	case definitions.DefFormatYAML, definitions.DefFormatJSON:
		return getTaskInfoFromDefn(ctx, cfg)
	default:
		return getTaskInfoFromScript(ctx, cfg)
	}
}

func getTaskInfoFromDefn(ctx context.Context, cfg taskDevConfig) (taskInfo, error) {
	dir, err := taskdir.Open(cfg.fileOrDir)
	if err != nil {
		return taskInfo{}, err
	}
	defer dir.Close()

	def, err := dir.ReadDefinition()
	if err != nil {
		return taskInfo{}, err
	}

	utr, err := def.GetUpdateTaskRequest(ctx, cfg.root.Client)
	if err != nil {
		return taskInfo{}, err
	}

	return taskInfo{
		slug:                def.GetSlug(),
		name:                def.GetName(),
		kind:                utr.Kind,
		kindOptions:         utr.KindOptions,
		parameters:          utr.Parameters,
		resourceAttachments: def.GetResourceAttachments(),
	}, nil
}

func getTaskInfoFromScript(ctx context.Context, cfg taskDevConfig) (taskInfo, error) {
	slug, err := utils.SlugFrom(cfg.fileOrDir)
	if err != nil {
		return taskInfo{}, err
	}

	task, err := cfg.root.Client.GetTask(ctx, libapi.GetTaskRequest{
		Slug:    slug,
		EnvSlug: cfg.envSlug,
	})
	if err != nil {
		return taskInfo{}, errors.Wrap(err, "getting task")
	}

	return taskInfo{
		slug:        task.Slug,
		name:        task.Name,
		kind:        task.Kind,
		kindOptions: task.KindOptions,
		parameters:  task.Parameters,
	}, nil
}
