package discover

import (
	"context"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	api "github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/runtime"
	_ "github.com/airplanedev/cli/pkg/runtime/builtin"
	_ "github.com/airplanedev/cli/pkg/runtime/image"
	_ "github.com/airplanedev/cli/pkg/runtime/javascript"
	_ "github.com/airplanedev/cli/pkg/runtime/python"
	_ "github.com/airplanedev/cli/pkg/runtime/rest"
	_ "github.com/airplanedev/cli/pkg/runtime/shell"
	_ "github.com/airplanedev/cli/pkg/runtime/sql"
	_ "github.com/airplanedev/cli/pkg/runtime/typescript"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

type ScriptDiscoverer struct {
	Client  api.IAPIClient
	Logger  logger.Logger
	EnvSlug string
}

var _ TaskDiscoverer = &ScriptDiscoverer{}

func (sd *ScriptDiscoverer) GetAirplaneTasks(ctx context.Context, file string) ([]string, error) {
	slug := runtime.Slug(file)
	if slug != "" {
		return []string{slug}, nil
	}
	return nil, nil
}

func (sd *ScriptDiscoverer) GetTaskConfigs(ctx context.Context, file string) ([]TaskConfig, error) {
	slug := runtime.Slug(file)
	if slug == "" {
		return nil, nil
	}

	task, err := sd.Client.GetTask(ctx, api.GetTaskRequest{
		Slug:    slug,
		EnvSlug: sd.EnvSlug,
	})
	if err != nil {
		var merr *api.TaskMissingError
		if !errors.As(err, &merr) {
			return nil, errors.Wrap(err, "unable to get task")
		}

		sd.Logger.Warning(`Task with slug %s does not exist, skipping deployment.`, slug)
		return nil, nil
	}
	if task.IsArchived {
		sd.Logger.Warning(`Task with slug %s is archived, skipping deployment.`, slug)
		return nil, nil
	}

	resp, err := sd.Client.ListResourceMetadata(ctx)
	if err != nil {
		return nil, err
	}

	def, err := definitions.NewDefinitionFromTask(task, resp.Resources)
	if err != nil {
		return nil, err
	}

	pathMetadata, err := taskPathMetadata(file, task.Kind)
	if err != nil {
		return nil, err
	}
	def.SetBuildConfig("entrypoint", pathMetadata.RelEntrypoint)
	if err := def.SetWorkdir(pathMetadata.RootDir, pathMetadata.WorkDir); err != nil {
		return nil, err
	}

	return []TaskConfig{
		{
			TaskID:         task.ID,
			TaskRoot:       pathMetadata.RootDir,
			TaskEntrypoint: pathMetadata.AbsEntrypoint,
			Def:            def,
			Source:         sd.ConfigSource(),
		},
	}, nil
}

func (sd *ScriptDiscoverer) GetTaskRoot(ctx context.Context, file string) (string, buildtypes.BuildContext, error) {
	slug := runtime.Slug(file)
	if slug == "" {
		return "", buildtypes.BuildContext{}, nil
	}

	task, err := sd.Client.GetTask(ctx, api.GetTaskRequest{
		Slug:    slug,
		EnvSlug: sd.EnvSlug,
	})
	if err != nil {
		var merr *api.TaskMissingError
		if !errors.As(err, &merr) {
			return "", buildtypes.BuildContext{}, nil
		}

		sd.Logger.Warning(`Task with slug %s does not exist, skipping deployment.`, slug)
		return "", buildtypes.BuildContext{}, nil
	}
	if task.IsArchived {
		sd.Logger.Warning(`Task with slug %s is archived, skipping deployment.`, slug)
		return "", buildtypes.BuildContext{}, nil
	}

	resp, err := sd.Client.ListResourceMetadata(ctx)
	if err != nil {
		return "", buildtypes.BuildContext{}, err
	}

	def, err := definitions.NewDefinitionFromTask(task, resp.Resources)
	if err != nil {
		return "", buildtypes.BuildContext{}, err
	}

	pathMetadata, err := taskPathMetadata(file, task.Kind)
	if err != nil {
		return "", buildtypes.BuildContext{}, err
	}

	bc, err := TaskBuildContext(pathMetadata.RootDir, pathMetadata.Runtime)
	if err != nil {
		return "", buildtypes.BuildContext{}, err
	}
	if err := def.SetBuildVersionBase(bc.Version, bc.Base); err != nil {
		return "", buildtypes.BuildContext{}, err
	}

	buildType, buildTypeVersion, buildBase, err := def.GetBuildType()
	if err != nil {
		return "", buildtypes.BuildContext{}, err
	}

	return pathMetadata.RootDir, buildtypes.BuildContext{
		Type:    buildType,
		Version: buildTypeVersion,
		Base:    buildBase,
		EnvVars: bc.EnvVars,
	}, nil
}

func (sd *ScriptDiscoverer) ConfigSource() ConfigSource {
	return ConfigSourceScript
}
