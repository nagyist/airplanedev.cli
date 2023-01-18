package discover

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	deployutils "github.com/airplanedev/lib/pkg/deploy/utils"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

//go:embed parser/node/parser.js
var nodeParserScript []byte

//go:embed parser/python/parser.py
var pythonParserScript string

type CodeTaskDiscoverer struct {
	Client api.IAPIClient
	Logger logger.Logger

	// MissingTaskHandler is called from `GetTaskConfig` if a task ID cannot be found for a definition
	// file. The handler should either create the task and return the created task's TaskMetadata, or
	// it should return `nil` to signal that the definition should be ignored. If not set, these
	// definitions are ignored.
	MissingTaskHandler func(context.Context, definitions.DefinitionInterface) (*api.TaskMetadata, error)

	// DoNotVerifyMissingTasks will return TaskConfigs for tasks without verifying their existence
	// in the api. If this value is set to true, MissingTaskHandler is ignored.
	DoNotVerifyMissingTasks bool
}

var _ TaskDiscoverer = &CodeTaskDiscoverer{}

func (c *CodeTaskDiscoverer) GetAirplaneTasks(ctx context.Context, file string) ([]string, error) {
	taskConfigs, err := c.GetTaskConfigs(ctx, file)
	if err != nil {
		return nil, err
	}

	var taskSlugs []string
	for _, taskConfig := range taskConfigs {
		taskSlugs = append(taskSlugs, taskConfig.Def.GetSlug())
	}
	return taskSlugs, nil
}

func (c *CodeTaskDiscoverer) GetTaskConfigs(ctx context.Context, file string) ([]TaskConfig, error) {
	if !deployutils.IsInlineAirplaneEntity(file) {
		return nil, nil
	}
	defs, err := c.parseDefinitions(ctx, file)
	if err != nil {
		return nil, err
	}

	var taskConfigs []TaskConfig
	for _, def := range defs {
		var metadata api.TaskMetadata
		if !c.DoNotVerifyMissingTasks {
			metadata, err = c.Client.GetTaskMetadata(ctx, def.Def.GetSlug())
			if err != nil {
				var merr *api.TaskMissingError
				if !errors.As(err, &merr) {
					return nil, errors.Wrap(err, "unable to get task metadata")
				}

				if c.MissingTaskHandler == nil {
					c.Logger.Warning(`Task with slug %s does not exist, skipping this task...`, def.Def.GetSlug())
					continue
				}

				mptr, err := c.MissingTaskHandler(ctx, def.Def)
				if err != nil {
					return nil, err
				} else if mptr == nil {
					c.Logger.Warning(`Task with slug %s does not exist, skipping this task...`, def.Def.GetSlug())
					continue
				}
				metadata = *mptr
			}
			if metadata.IsArchived {
				c.Logger.Warning(`Task with slug %s is archived, skipping this task...`, metadata.Slug)
				continue
			}
		}

		taskConfigs = append(taskConfigs, TaskConfig{
			TaskID:         metadata.ID,
			TaskRoot:       def.PathMetadata.RootDir,
			TaskEntrypoint: def.PathMetadata.AbsEntrypoint,
			Def:            def.Def,
			Source:         ConfigSourceCode,
		})
	}
	return taskConfigs, nil
}

func (c *CodeTaskDiscoverer) GetTaskRoot(ctx context.Context, file string) (string, build.BuildContext, error) {
	if !deployutils.IsInlineAirplaneEntity(file) {
		return "", build.BuildContext{}, nil
	}

	var kind build.TaskKind
	var buildType build.BuildType
	if deployutils.IsNodeInlineAirplaneEntity(file) {
		kind = build.TaskKindNode
		buildType = build.NodeBuildType
	} else if deployutils.IsPythonInlineAirplaneEntity(file) {
		kind = build.TaskKindPython
		buildType = build.PythonBuildType
	}
	if kind == "" {
		return "", build.BuildContext{}, nil
	}
	pm, err := taskPathMetadata(file, kind)
	if err != nil {
		return "", build.BuildContext{}, errors.Wrap(err, "unable to interpret task path metadata")
	}
	bc, err := TaskBuildContext(pm.RootDir, pm.Runtime)
	if err != nil {
		return "", build.BuildContext{}, err
	}
	base := bc.Base
	if base == build.BuildBaseNone {
		// Default to the slim base if otherwise unspecified.
		base = build.BuildBaseSlim
	}

	return pm.RootDir, build.BuildContext{
		Type:    buildType,
		Version: bc.Version,
		Base:    base,
		EnvVars: bc.EnvVars,
	}, nil
}

func (c *CodeTaskDiscoverer) ConfigSource() ConfigSource {
	return ConfigSourceCode
}

type ParsedDefinition struct {
	Def          definitions.DefinitionInterface
	PathMetadata TaskPathMetadata
}

func (c *CodeTaskDiscoverer) parseDefinitions(ctx context.Context, file string) ([]ParsedDefinition, error) {
	if deployutils.IsNodeInlineAirplaneEntity(file) {
		return c.parseNodeDefinitions(ctx, file)
	} else if deployutils.IsPythonInlineAirplaneEntity(file) {
		return c.parsePythonDefinitions(ctx, file)
	}
	return nil, nil
}

func (c *CodeTaskDiscoverer) parseNodeDefinitions(ctx context.Context, file string) ([]ParsedDefinition, error) {
	pm, err := taskPathMetadata(file, build.TaskKindNode)
	if err != nil {
		return nil, errors.Wrap(err, "unable to interpret task path metadata")
	}
	bc, err := TaskBuildContext(pm.RootDir, pm.Runtime)
	if err != nil {
		return nil, err
	}

	if err := esbuildUserFiles(c.Logger, pm.RootDir); err != nil {
		// TODO: convert to an error once inline discovery is more stable.
		c.Logger.Log("")
		c.Logger.Warning(`Unable to build task: %s`, err.Error())
		return nil, nil
	}
	defer func() {
		if err := os.RemoveAll(path.Join(pm.RootDir, ".airplane", "discover")); err != nil {
			c.Logger.Warning("unable to remove temporary directory: %s")
		}
	}()

	compiledJSPath, err := compiledFilePath(pm.RootDir, file)
	if err != nil {
		return nil, err
	}

	parsedConfigs, err := extractJSConfigs(compiledJSPath)
	if err != nil {
		c.Logger.Warning(`Unable to discover inline configured tasks: %s`, err.Error())
	}

	var parsedDefinitions []ParsedDefinition
	for _, parsedTask := range parsedConfigs.TaskConfigs {
		// Add the entrypoint to the json definition before validation
		// since it is unknown to the parser.
		nodeConfig := parsedTask["node"].(map[string]interface{})
		nodeConfig["entrypoint"] = pm.RelEntrypoint

		def, err := ConstructDefinition(parsedTask, pm, bc)
		if err != nil {
			return nil, err
		}

		parsedDefinitions = append(parsedDefinitions, ParsedDefinition{
			Def:          def,
			PathMetadata: pm,
		})
	}

	return parsedDefinitions, nil
}

func (c *CodeTaskDiscoverer) parsePythonDefinitions(ctx context.Context, file string) ([]ParsedDefinition, error) {
	parsedConfigs, err := extractPythonConfigs(file)
	if err != nil {
		c.Logger.Warning(`Unable to discover inline configured tasks: %s`, err.Error())
	}

	pathMetadata, err := taskPathMetadata(file, build.TaskKindPython)
	if err != nil {
		return nil, err
	}
	bc, err := TaskBuildContext(pathMetadata.RootDir, pathMetadata.Runtime)
	if err != nil {
		return nil, err
	}

	var parsedDefinitions []ParsedDefinition
	for _, parsedTask := range parsedConfigs {
		// Add the entrypoint to the json definition before validation
		// since it is unknown to the parser.
		pythonConfig := parsedTask["python"].(map[string]interface{})
		pythonConfig["entrypoint"] = pathMetadata.RelEntrypoint

		def, err := ConstructDefinition(parsedTask, pathMetadata, bc)
		if err != nil {
			return nil, err
		}

		parsedDefinitions = append(parsedDefinitions, ParsedDefinition{
			Def:          def,
			PathMetadata: pathMetadata,
		})
	}

	return parsedDefinitions, nil
}

func ConstructDefinition(parsedTask map[string]interface{}, pathMetadata TaskPathMetadata, buildContext build.BuildContext) (definitions.DefinitionInterface, error) {
	entrypointFunc, ok := parsedTask["entrypointFunc"].(string)
	if !ok {
		return nil, errors.New("expected 'entrypointFunc' key in parsed task")
	}
	delete(parsedTask, "entrypointFunc")

	def := definitions.Definition_0_3{}
	b, err := json.Marshal(parsedTask)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize task json properly")
	}

	if err := def.Unmarshal(definitions.DefFormatJSON, b); err != nil {
		switch err := errors.Cause(err).(type) {
		case definitions.ErrSchemaValidation:
			errorMsgs := []string{}
			for _, verr := range err.Errors {
				errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", verr.Field(), verr.Description()))
			}
			return nil, definitions.NewErrReadDefinition(fmt.Sprintf("Error reading %s", pathMetadata.AbsEntrypoint), errorMsgs...)
		default:
			return nil, errors.Wrap(err, "unmarshalling task definition")
		}
	}

	def.SetBuildConfig("entrypoint", pathMetadata.RelEntrypoint)
	def.SetBuildConfig("entrypointFunc", entrypointFunc)

	if err := def.SetWorkdir(pathMetadata.RootDir, pathMetadata.WorkDir); err != nil {
		return nil, err
	}

	if err := def.SetAbsoluteEntrypoint(pathMetadata.AbsEntrypoint); err != nil {
		return nil, err
	}
	// Code based tasks do not have the concept of an entrypoint.
	if err := def.SetEntrypoint(""); err != nil {
		return nil, err
	}
	if err := def.SetBuildVersionBase(buildContext.Version, buildContext.Base); err != nil {
		return nil, err
	}
	envVars := make(api.TaskEnv)
	envVarsFromDefn, err := def.GetEnv()
	if err != nil {
		return nil, err
	}
	// Calculate the full list of env vars. This is the env vars (from airplane config)
	// plus the env vars from the task. Set this new list on the task def.
	for k, v := range buildContext.EnvVars {
		envVars[k] = api.EnvVarValue(v)
	}
	for k, v := range envVarsFromDefn {
		envVars[k] = v
	}
	if len(envVars) > 0 {
		if err := def.SetEnv(envVars); err != nil {
			return nil, err
		}
	}

	def.SetDefnFilePath(pathMetadata.AbsEntrypoint)

	return &def, nil
}
