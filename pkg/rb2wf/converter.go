package rb2wf

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/logger"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/pkg/errors"
)

var (
	//go:embed templates/blocks/email.ts.tmpl
	emailTemplate string

	//go:embed templates/blocks/graphql.ts.tmpl
	graphQLTemplate string

	//go:embed templates/main.airplane.ts.tmpl
	mainTemplate string

	//go:embed templates/blocks/mongodb.ts.tmpl
	mongoDBTemplate string

	//go:embed templates/blocks/note.ts.tmpl
	noteTemplate string

	//go:embed static/.prettierrc
	prettierConfig string

	//go:embed templates/blocks/rest.ts.tmpl
	restTemplate string

	//go:embed templates/blocks/slack.ts.tmpl
	slackTemplate string

	//go:embed templates/blocks/sql.ts.tmpl
	sqlTemplate string

	//go:embed templates/blocks/task.ts.tmpl
	taskTemplate string

	//go:embed templates/blocks/unimplemented.ts.tmpl
	unimplementedTemplate string
)

type RunbookConverter struct {
	client     api.APIClient
	entrypoint string
	outputDir  string
}

func NewRunbookConverter(
	client api.APIClient,
	outputDir string,
	entrypoint string,
) *RunbookConverter {
	return &RunbookConverter{
		client:     client,
		entrypoint: entrypoint,
		outputDir:  outputDir,
	}
}

func (r *RunbookConverter) Convert(ctx context.Context, runbookSlug string) error {
	logger.Step("Fetching details of runbook %s", runbookSlug)
	runbookInfo, err := r.getRunbookInfo(ctx, runbookSlug)
	if err != nil {
		return err
	}

	// TODO: Use template from
	// https://github.com/airplanedev/lib/blob/main/pkg/runtime/javascript/javascript.go#L88
	// for standard inline parameters (params, constraints, etc.).
	workflowParams := map[string]map[string]interface{}{}
	for _, param := range runbookInfo.runbook.Runbook.Parameters {
		workflowParams[param.Slug] = map[string]interface{}{
			"name": param.Name,
			"slug": param.Slug,
			"type": typeToWorkflowType(param.Type),
		}

		if param.Default != nil {
			workflowParams[param.Slug]["default"] = param.Default
		}
	}

	envVars := map[string]map[string]interface{}{}
	configs := map[string]string{}

	for _, configVar := range runbookInfo.runbook.Runbook.TemplateSession.Configs {
		envVars[configVar.NameTag] = map[string]interface{}{
			"config": configVar.NameTag,
		}
		configs[configVar.NameTag] = fmt.Sprintf(
			"%sprocess.env['%s']%s",
			noQuoteStart,
			configVar.NameTag,
			noQuoteEnd,
		)
	}

	logger.Step("Getting resources associated with the runbook")
	resources, err := r.getResources(ctx, runbookInfo.blocks.Blocks)
	if err != nil {
		return err
	}
	resourceSlugs := []string{}
	for _, resource := range resources {
		resourceSlugs = append(resourceSlugs, resource.Slug)
	}

	sort.Slice(resourceSlugs, func(a, b int) bool {
		return resourceSlugs[a] < resourceSlugs[b]
	})

	bodyChunks := []string{}

	for _, block := range runbookInfo.blocks.Blocks {
		logger.Step("Converting block %s to code", block.Slug)
		blockStr, err := r.blockToString(ctx, block, resources)
		if err != nil {
			return err
		}
		bodyChunks = append(bodyChunks, blockStr)
	}

	logger.Step("Creating %s", r.entrypoint)
	mainTsStr, err := applyTemplate(mainTemplate, struct {
		Body        string
		Configs     interface{}
		Constraints interface{}
		EnvVars     interface{}
		Name        string
		Parameters  interface{}
		Resources   []string
		RunbookID   string
		RunbookSlug string
		Slug        string
	}{
		Body:        strings.Join(bodyChunks, "\n"),
		Configs:     configs,
		Constraints: constraintsToMap(runbookInfo.runbook.Runbook.TemplateSession.Constraints),
		EnvVars:     envVars,
		Name:        fmt.Sprintf("%s (workflow)", runbookInfo.runbook.Runbook.Name),
		Parameters:  workflowParams,
		Resources:   resourceSlugs,
		RunbookID:   runbookInfo.runbook.Runbook.ID,
		RunbookSlug: runbookInfo.runbook.Runbook.Slug,
		Slug:        runbookInfo.runbook.Runbook.Slug,
	})
	if err != nil {
		return err
	}
	if err := r.writeFile(ctx, r.entrypoint, mainTsStr); err != nil {
		return err
	}

	logger.Step("Formatting code via prettier")
	if err := r.writeFile(ctx, ".prettierrc", prettierConfig); err != nil {
		return err
	}
	if err := r.runPrettier(ctx); err != nil {
		return err
	}

	return nil
}

type runbookInfo struct {
	runbook api.GetRunbookResponse
	blocks  api.ListSessionBlocksResponse
}

func (r *RunbookConverter) getRunbookInfo(
	ctx context.Context,
	runbookSlug string,
) (*runbookInfo, error) {
	runbookResp, err := r.client.GetRunbook(ctx, runbookSlug)
	if err != nil {
		return nil, err
	}

	sessionBlocksResp, err := r.client.ListSessionBlocks(
		ctx,
		runbookResp.Runbook.TemplateSession.ID,
	)
	if err != nil {
		return nil, err
	}

	return &runbookInfo{
		runbook: runbookResp,
		blocks:  sessionBlocksResp,
	}, nil
}

func (r *RunbookConverter) blockToString(
	ctx context.Context,
	block api.SessionBlock,
	resources map[string]libapi.Resource,
) (string, error) {
	// TODO: Add support for form blocks.
	if block.BlockKindConfig.Task != nil {
		config := block.BlockKindConfig.Task
		task, err := r.client.GetTaskByID(ctx, config.TaskID)
		if err != nil {
			return "", err
		}
		taskSlug := task.Slug

		taskParamValues, err := interfaceToJSObj(config.ParamValues, nil)
		if err != nil {
			return "", err
		}

		taskBlockStr, err := applyTemplate(taskTemplate, struct {
			BlockSlug      string
			ParamValues    interface{}
			StartCondition string
			TaskSlug       string
		}{
			BlockSlug:      block.Slug,
			ParamValues:    taskParamValues,
			StartCondition: block.StartCondition,
			TaskSlug:       taskSlug,
		})
		if err != nil {
			return "", err
		}

		return taskBlockStr, nil
	} else if block.BlockKindConfig.Note != nil {
		config := block.BlockKindConfig.Note

		noteContent, err := interfaceToJSObj(config.Content, nil)
		if err != nil {
			return "", err
		}

		noteBlockStr, err := applyTemplate(noteTemplate, struct {
			BlockSlug      string
			Content        interface{}
			StartCondition string
		}{
			BlockSlug:      block.Slug,
			Content:        noteContent,
			StartCondition: block.StartCondition,
		})
		if err != nil {
			return "", err
		}

		return noteBlockStr, nil
	} else if block.BlockKindConfig.StdAPI != nil {
		config := block.BlockKindConfig.StdAPI

		req, ok := config.Request.(map[string]interface{})
		if !ok {
			return "", errors.Errorf("stdapi request was not map: %+v", config.Request)
		}

		switch config.Namespace {
		case "email":
			sender, err := interfaceToJSObj(req["sender"], nil)
			if err != nil {
				return "", err
			}
			receipients, err := interfaceToJSObj(req["recipients"], nil)
			if err != nil {
				return "", err
			}
			options, err := interfaceToJSObj(
				req, map[string]struct{}{
					"sender":     {},
					"recipients": {},
				},
			)
			if err != nil {
				return "", err
			}

			resourceID := firstValue(config.Resources)
			resourceSlug := resources[resourceID].Slug
			emailBlockStr, err := applyTemplate(emailTemplate, struct {
				BlockSlug      string
				Options        interface{}
				Recipients     interface{}
				ResourceSlug   string
				Sender         interface{}
				StartCondition string
			}{
				BlockSlug:      block.Slug,
				Options:        options,
				Recipients:     receipients,
				ResourceSlug:   resourceSlug,
				Sender:         sender,
				StartCondition: block.StartCondition,
			})
			if err != nil {
				return "", err
			}
			return emailBlockStr, nil
		case "graphql":
			operation, err := interfaceToJSObj(req["operation"], nil)
			if err != nil {
				return "", err
			}

			options, err := interfaceToJSObj(
				req, map[string]struct{}{
					"operation": {},
				},
			)
			if err != nil {
				return "", err
			}

			resourceID := firstValue(config.Resources)
			resourceSlug := resources[resourceID].Slug
			graphQLBlockStr, err := applyTemplate(graphQLTemplate, struct {
				BlockSlug      string
				Operation      interface{}
				Options        interface{}
				ResourceSlug   string
				StartCondition string
			}{
				BlockSlug:      block.Slug,
				Operation:      operation,
				Options:        options,
				ResourceSlug:   resourceSlug,
				StartCondition: block.StartCondition,
			})
			if err != nil {
				return "", err
			}
			return graphQLBlockStr, nil
		case "mongodb":
			collection, err := interfaceToJSObj(req["collection"], nil)
			if err != nil {
				return "", err
			}

			options, err := interfaceToJSObj(
				req, map[string]struct{}{
					"collection": {},
				},
			)
			if err != nil {
				return "", err
			}

			resourceID := firstValue(config.Resources)
			resourceSlug := resources[resourceID].Slug
			mongoDBBlockStr, err := applyTemplate(mongoDBTemplate, struct {
				BlockSlug      string
				Collection     interface{}
				DBSlug         string
				Operation      string
				Options        interface{}
				StartCondition string
			}{
				BlockSlug:      block.Slug,
				Collection:     collection,
				DBSlug:         resourceSlug,
				Operation:      config.Name,
				Options:        options,
				StartCondition: block.StartCondition,
			})
			if err != nil {
				return "", err
			}
			return mongoDBBlockStr, nil
		case "rest":
			method, err := interfaceToJSObj(req["method"], nil)
			if err != nil {
				return "", err
			}
			path, err := interfaceToJSObj(req["path"], nil)
			if err != nil {
				return "", err
			}

			options, err := interfaceToJSObj(
				req, map[string]struct{}{
					"method": {},
					"path":   {},
				},
			)
			if err != nil {
				return "", err
			}

			resourceID := firstValue(config.Resources)
			resourceSlug := resources[resourceID].Slug
			restBlockStr, err := applyTemplate(restTemplate, struct {
				BlockSlug      string
				Method         interface{}
				Options        interface{}
				Path           interface{}
				ResourceSlug   string
				StartCondition string
			}{
				BlockSlug:      block.Slug,
				Method:         method,
				Options:        options,
				Path:           path,
				ResourceSlug:   resourceSlug,
				StartCondition: block.StartCondition,
			})
			if err != nil {
				return "", err
			}
			return restBlockStr, nil
		case "slack":
			channel, err := interfaceToJSObj(req["channelName"], nil)
			if err != nil {
				return "", err
			}
			message, err := interfaceToJSObj(req["message"], nil)
			if err != nil {
				return "", err
			}

			slackBlockStr, err := applyTemplate(slackTemplate, struct {
				BlockSlug      string
				Channel        interface{}
				Message        interface{}
				StartCondition string
			}{
				BlockSlug:      block.Slug,
				Channel:        channel,
				Message:        message,
				StartCondition: block.StartCondition,
			})
			if err != nil {
				return "", err
			}
			return slackBlockStr, nil
		case "sql":
			query, err := interfaceToJSObj(req["query"], nil)
			if err != nil {
				return "", err
			}

			queryArgs, err := interfaceToJSObj(req["queryArgs"], nil)
			if err != nil {
				return "", err
			}

			dbID := firstValue(config.Resources)
			dbSlug := resources[dbID].Slug
			queryBlockStr, err := applyTemplate(sqlTemplate, struct {
				BlockSlug      string
				DBSlug         string
				Query          interface{}
				QueryArgs      interface{}
				StartCondition string
			}{
				BlockSlug:      block.Slug,
				DBSlug:         dbSlug,
				Query:          query,
				QueryArgs:      queryArgs,
				StartCondition: block.StartCondition,
			})
			if err != nil {
				return "", err
			}
			return queryBlockStr, nil
		}
	}

	// Something not yet implemented
	blockBytes, err := json.MarshalIndent(block, "", " ")
	if err != nil {
		return "", err
	}

	unimplementedBlockStr, err := applyTemplate(unimplementedTemplate, struct {
		BlockSlug string
		BlockStr  string
	}{
		BlockSlug: block.Slug,
		BlockStr:  string(blockBytes),
	})
	if err != nil {
		return "", err
	}
	return unimplementedBlockStr, nil
}

func (r *RunbookConverter) getResources(
	ctx context.Context,
	blocks []api.SessionBlock,
) (map[string]libapi.Resource, error) {
	resources := map[string]libapi.Resource{}

	for _, block := range blocks {
		if block.BlockKindConfig.StdAPI != nil {
			config := block.BlockKindConfig.StdAPI
			for _, resourceID := range config.Resources {
				resourceResp, err := r.client.GetResource(
					ctx,
					api.GetResourceRequest{ID: resourceID},
				)
				if err != nil {
					return nil, errors.Wrap(err, "getting resource from API")
				}

				resources[resourceResp.ID] = resourceResp.Resource
			}
		}
	}

	return resources, nil
}

func (r *RunbookConverter) writeFile(
	ctx context.Context,
	relPath string,
	contents string,
) error {
	return os.WriteFile(
		filepath.Join(r.outputDir, relPath),
		[]byte(contents),
		0644,
	)
}

func (r *RunbookConverter) runPrettier(ctx context.Context) error {
	cmd := exec.Command("npx", "--yes", "prettier", "-w", "./*ts")
	cmd.Dir = r.outputDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log("prettier output: %s", string(out))
		return errors.Wrap(err, "running prettier")
	}

	return nil
}
