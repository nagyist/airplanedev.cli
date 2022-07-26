package discover

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

//go:embed parser/node/parser.ts
var parserScript []byte

type CodeTaskDiscoverer struct {
	Client api.IAPIClient
	Logger logger.Logger

	// MissingTaskHandler is called from `GetTaskConfig` if a task ID cannot be found for a definition
	// file. The handler should either create the task and return the created task's TaskMetadata, or
	// it should return `nil` to signal that the definition should be ignored. If not set, these
	// definitions are ignored.
	MissingTaskHandler func(context.Context, definitions.DefinitionInterface) (*api.TaskMetadata, error)
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
	if !(strings.HasSuffix(file, ".task.ts") || strings.HasSuffix(file, ".task.js")) {
		return nil, nil
	}

	// Create a temp file
	tempFile, err := os.CreateTemp("", "airplane.parser.node.*.ts")
	if err != nil {
		return nil, nil
	}
	defer os.Remove(tempFile.Name())
	_, err = tempFile.Write(parserScript)
	if err != nil {
		return nil, err
	}

	// Run parser on the file
	out, err := exec.Command("npx", "-p", "typescript", "-p", "@types/node", "-p", "ts-node",
		"ts-node", tempFile.Name(), file).Output()
	if err != nil {
		return nil, err
	}

	var parsedTasks []map[string]interface{}
	if err := json.Unmarshal(out, &parsedTasks); err != nil {
		return nil, err
	}

	if len(parsedTasks) == 0 {
		// Unable to find any Airplane tasks in the file
		return nil, nil
	}

	pathMetadata, err := taskPathMetadata(file, build.TaskKindNode)
	if err != nil {
		return nil, err
	}

	var taskConfigs []TaskConfig
	for _, parsedTask := range parsedTasks {
		// Modify fields to add ones that are strictly required
		entrypointFunc := parsedTask["entrypointFunc"].(string)
		delete(parsedTask, "entrypointFunc")
		parsedTask["node"] = map[string]interface{}{
			"nodeVersion": build.LatestNodeVersion,
			"entrypoint":  pathMetadata.RelEntrypoint,
		}

		// Construct task config
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
				return nil, definitions.NewErrReadDefinition(fmt.Sprintf("Error reading %s", file), errorMsgs...)
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

		// Task metadata
		metadata, err := c.Client.GetTaskMetadata(ctx, def.GetSlug())
		if err != nil {
			var merr *api.TaskMissingError
			if !errors.As(err, &merr) {
				return nil, errors.Wrap(err, "unable to get task metadata")
			}

			if c.MissingTaskHandler == nil {
				c.Logger.Warning(`Task with slug %s does not exist, skipping this task...`, def.GetSlug())
				continue
			}

			mptr, err := c.MissingTaskHandler(ctx, &def)
			if err != nil {
				return nil, err
			} else if mptr == nil {
				c.Logger.Warning(`Task with slug %s does not exist, skipping this task...`, def.GetSlug())
				continue
			}
			metadata = *mptr
		}
		if metadata.IsArchived {
			c.Logger.Warning(`Task with slug %s is archived, skipping this task...`, metadata.Slug)
			continue
		}

		taskConfigs = append(taskConfigs, TaskConfig{
			TaskID:         metadata.ID,
			TaskRoot:       pathMetadata.RootDir,
			TaskEntrypoint: pathMetadata.AbsEntrypoint,
			Def:            &def,
			Source:         ConfigSourceCode,
		})
	}

	return taskConfigs, nil
}

func (c *CodeTaskDiscoverer) ConfigSource() ConfigSource {
	return ConfigSourceCode
}
