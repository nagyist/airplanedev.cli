package discover

import (
	"context"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/runtime"
	_ "github.com/airplanedev/lib/pkg/runtime/builtin"
	_ "github.com/airplanedev/lib/pkg/runtime/image"
	_ "github.com/airplanedev/lib/pkg/runtime/javascript"
	_ "github.com/airplanedev/lib/pkg/runtime/python"
	_ "github.com/airplanedev/lib/pkg/runtime/rest"
	_ "github.com/airplanedev/lib/pkg/runtime/shell"
	_ "github.com/airplanedev/lib/pkg/runtime/sql"
	_ "github.com/airplanedev/lib/pkg/runtime/typescript"
	"github.com/airplanedev/lib/pkg/utils/logger"
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

	def, err := definitions.NewDefinitionFromTask(ctx, sd.Client, task)
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

func (sd *ScriptDiscoverer) GetTaskRoot(ctx context.Context, file string) (string, build.BuildContext, error) {
	slug := runtime.Slug(file)
	if slug == "" {
		return "", build.BuildContext{}, nil
	}

	task, err := sd.Client.GetTask(ctx, api.GetTaskRequest{
		Slug:    slug,
		EnvSlug: sd.EnvSlug,
	})
	if err != nil {
		var merr *api.TaskMissingError
		if !errors.As(err, &merr) {
			return "", build.BuildContext{}, nil
		}

		sd.Logger.Warning(`Task with slug %s does not exist, skipping deployment.`, slug)
		return "", build.BuildContext{}, nil
	}
	if task.IsArchived {
		sd.Logger.Warning(`Task with slug %s is archived, skipping deployment.`, slug)
		return "", build.BuildContext{}, nil
	}

	def, err := definitions.NewDefinitionFromTask(ctx, sd.Client, task)
	if err != nil {
		return "", build.BuildContext{}, err
	}

	pathMetadata, err := taskPathMetadata(file, task.Kind)
	if err != nil {
		return "", build.BuildContext{}, err
	}

	bc, err := TaskBuildContext(pathMetadata.RootDir, pathMetadata.Runtime)
	if err != nil {
		return "", build.BuildContext{}, err
	}
	if err := def.SetBuildVersionBase(bc.Version, bc.Base); err != nil {
		return "", build.BuildContext{}, err
	}

	buildType, buildTypeVersion, buildBase, err := def.GetBuildType()
	if err != nil {
		return "", build.BuildContext{}, err
	}

	return pathMetadata.RootDir, build.BuildContext{
		Type:    buildType,
		Version: buildTypeVersion,
		Base:    buildBase,
		EnvVars: bc.EnvVars,
	}, nil
}

func (sd *ScriptDiscoverer) ConfigSource() ConfigSource {
	return ConfigSourceScript
}
