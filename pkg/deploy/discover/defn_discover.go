package discover

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

type DefnDiscoverer struct {
	Client api.IAPIClient
	Logger logger.Logger

	// MissingTaskHandler is called from `GetTaskConfig` if a task ID cannot be found for a definition
	// file. The handler should either create the task and return the created task's TaskMetadata, or
	// it should return `nil` to signal that the definition should be ignored. If not set, these
	// definitions are ignored.
	MissingTaskHandler func(context.Context, definitions.DefinitionInterface) (*api.TaskMetadata, error)

	// DisableNormalize is used to determine whether to Normalize a discovered task definition. Ideally, normalization
	// should be done in the deploy path, and not the discover path, but we include this flag for now so that certain
	// clients (e.g. studio) can skip some validation checks.
	// TODO: Remove this when we remove task diffs.
	DisableNormalize bool

	// DoNotVerifyMissingTasks will return TaskConfigs for tasks without verifying their existence
	// in the api. If this value is set to true, MissingTaskHandler is ignored.
	DoNotVerifyMissingTasks bool
}

var _ TaskDiscoverer = &DefnDiscoverer{}

func (dd *DefnDiscoverer) GetAirplaneTasks(ctx context.Context, file string) ([]string, error) {
	if !definitions.IsTaskDef(file) {
		return nil, nil
	}

	dir, err := taskdir.Open(file)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	def, err := dir.ReadDefinition()
	if err != nil {
		return nil, err
	}

	return []string{def.GetSlug()}, nil
}

func (dd *DefnDiscoverer) GetTaskConfigs(ctx context.Context, file string) ([]TaskConfig, error) {
	if !definitions.IsTaskDef(file) {
		siblingDef := searchTaskDefnInSibling(file)
		if siblingDef != "" {
			return dd.GetTaskConfigs(ctx, siblingDef)
		}
		return nil, nil
	}

	dir, err := taskdir.Open(file)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	def, err := dir.ReadDefinition()
	if err != nil {
		return nil, err
	}

	if !dd.DisableNormalize {
		if err := def.Normalize(ctx, dd.Client); err != nil {
			return nil, err
		}
	}

	tc := TaskConfig{
		Def:    def,
		Source: dd.ConfigSource(),
	}

	var metadata api.TaskMetadata
	if !dd.DoNotVerifyMissingTasks {
		metadata, err = dd.Client.GetTaskMetadata(ctx, def.GetSlug())
		if err != nil {
			var merr *api.TaskMissingError
			if !errors.As(err, &merr) {
				return nil, errors.Wrap(err, "unable to get task metadata")
			}

			if dd.MissingTaskHandler == nil {
				return nil, nil
			}

			mptr, err := dd.MissingTaskHandler(ctx, def)
			if err != nil {
				return nil, err
			} else if mptr == nil {
				if dd.Logger != nil {
					dd.Logger.Warning(`Task with slug %s does not exist, skipping deployment.`, def.GetSlug())
				}
				return nil, nil
			}
			metadata = *mptr
		}
		if metadata.IsArchived {
			dd.Logger.Warning(`Task with slug %s is archived, skipping deployment.`, metadata.Slug)
			return nil, nil
		}
	}
	tc.TaskID = metadata.ID

	root, _, err := setBuildVersionAndWorkingDir(file, def)
	if err != nil {
		return nil, err
	}
	tc.TaskRoot = root

	entrypoint, err := def.GetAbsoluteEntrypoint()
	if err == definitions.ErrNoEntrypoint {
		return []TaskConfig{tc}, nil
	} else if err != nil {
		return nil, err
	} else if err = fsx.AssertExistsAll(entrypoint); err != nil {
		return nil, err
	} else {
		tc.TaskEntrypoint = entrypoint

		// Entrypoint for builder needs to be relative to taskroot, not definition directory.
		defnDir := filepath.Dir(dir.DefinitionPath())
		if defnDir != tc.TaskRoot {
			ep, err := filepath.Rel(tc.TaskRoot, entrypoint)
			if err != nil {
				return nil, err
			}
			def.SetBuildConfig("entrypoint", ep)
		}
	}

	return []TaskConfig{tc}, nil
}

func (dd *DefnDiscoverer) GetTaskRoot(ctx context.Context, file string) (string, build.BuildContext, error) {
	if !definitions.IsTaskDef(file) {
		siblingDef := searchTaskDefnInSibling(file)
		if siblingDef != "" {
			return dd.GetTaskRoot(ctx, siblingDef)
		}
		return "", build.BuildContext{}, nil
	}

	dir, err := taskdir.Open(file)
	if err != nil {
		return "", build.BuildContext{}, err
	}
	defer dir.Close()

	def, err := dir.ReadDefinition()
	if err != nil {
		return "", build.BuildContext{}, err
	}

	return setBuildVersionAndWorkingDir(file, def)
}

func (dd *DefnDiscoverer) ConfigSource() ConfigSource {
	return ConfigSourceDefn
}

func setBuildVersionAndWorkingDir(file string, def definitions.DefinitionInterface) (string, build.BuildContext, error) {
	entrypoint, err := def.GetAbsoluteEntrypoint()
	if err == definitions.ErrNoEntrypoint {
		absFile, err := filepath.Abs(file)
		if err != nil {
			return "", build.BuildContext{}, err
		}
		entrypoint = absFile
	} else if err != nil {
		return "", build.BuildContext{}, err
	}
	if err = fsx.AssertExistsAll(entrypoint); err != nil {
		return "", build.BuildContext{}, err
	}
	kind, _, err := def.GetKindAndOptions()
	if err != nil {
		return "", build.BuildContext{}, err
	}

	taskPathMetadata, err := taskPathMetadata(entrypoint, kind)
	if err != nil {
		return "", build.BuildContext{}, err
	}
	bc, err := TaskBuildContext(taskPathMetadata.RootDir, taskPathMetadata.Runtime)
	if err != nil {
		return "", build.BuildContext{}, err
	}
	if err := def.SetBuildVersionBase(bc.Version, bc.Base); err != nil {
		return "", build.BuildContext{}, err
	}
	if err := def.SetWorkdir(taskPathMetadata.RootDir, taskPathMetadata.WorkDir); err != nil {
		return "", build.BuildContext{}, err
	}
	// Recalculate the build types.
	buildType, buildTypeVersion, buildBase, err := def.GetBuildType()
	if err != nil {
		return "", build.BuildContext{}, err
	}

	// Calculate the full list of env vars. This is the env vars (from airplane config)
	// plus the env vars from the task. Set this new list on the task def
	// and on the build context.
	envVars := make(map[string]build.EnvVarValue)
	envVarsFromDefn, err := def.GetEnv()
	if err != nil {
		return "", build.BuildContext{}, err
	}
	for k, v := range bc.EnvVars {
		envVars[k] = v
	}
	for k, v := range envVarsFromDefn {
		envVars[k] = build.EnvVarValue(v)
	}
	if len(envVars) == 0 {
		envVars = nil
	} else {
		newDefnEnvVars := make(api.TaskEnv, len(envVars))
		for k, v := range envVars {
			newDefnEnvVars[k] = api.EnvVarValue(v)
		}
		if err := def.SetEnv(newDefnEnvVars); err != nil {
			return "", build.BuildContext{}, err
		}
	}

	return taskPathMetadata.RootDir, build.BuildContext{
		Type:    buildType,
		Version: buildTypeVersion,
		Base:    buildBase,
		EnvVars: envVars,
	}, nil
}

func searchTaskDefnInSibling(file string) string {
	fileWithoutExtension := strings.TrimSuffix(file, filepath.Ext(file))
	for _, tde := range definitions.TaskDefExtensions {
		fileWithTaskDefExtension := fileWithoutExtension + tde
		if fsx.Exists(fileWithTaskDefExtension) {
			return fileWithTaskDefExtension
		}
	}
	return ""
}
