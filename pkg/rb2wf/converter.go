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

	libapi "github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/ojson"
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

	//go:embed templates/blocks/form.ts.tmpl
	formTemplate string

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

func (r *RunbookConverter) Convert(ctx context.Context, runbookSlug string, envSlug string) error {
	logger.Step("Fetching details of runbook %s", runbookSlug)
	runbookInfo, err := r.getRunbookInfo(ctx, runbookSlug, envSlug)
	if err != nil {
		return err
	}

	// TODO: Use template from
	// https://github.com/airplanedev/cli/blob/main/pkg/runtime/javascript/javascript.go#L88
	// for standard inline parameters (params, constraints, etc.).
	workflowParams := map[string]map[string]interface{}{}
	for _, param := range runbookInfo.runbook.Runbook.Parameters {
		workflowParams[param.Slug] = map[string]interface{}{
			"name": param.Name,
			"slug": param.Slug,
			"type": getParamType(param.Type, param.Component),
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
		blockStr, err := r.blockToString(ctx, block, runbookInfo, resources)
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
		Name:        fmt.Sprintf("%s (task)", runbookInfo.runbook.Runbook.Name),
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

	if strings.Contains(mainTsStr, manualFixPlaceholder) {
		logger.Warning("Some template expressions use features that are not supported by tasks. Search for instances of %q and update the code accordingly.", manualFixPlaceholder)
		logger.Warning("See %s for more details.", logger.Purple("https://docs.airplane.dev/runbooks/migrate-to-tasks#migrating-runbook-features-to-task-features"))
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
	envSlug string,
) (runbookInfo, error) {
	runbookResp, err := r.client.GetRunbook(ctx, runbookSlug, envSlug)
	if err != nil {
		return runbookInfo{}, err
	}

	sessionBlocksResp, err := r.client.ListSessionBlocks(
		ctx,
		runbookResp.Runbook.TemplateSession.ID,
	)
	if err != nil {
		return runbookInfo{}, err
	}

	return runbookInfo{
		runbook: runbookResp,
		blocks:  sessionBlocksResp,
	}, nil
}

// PromptAPIParameter maps to the json struct for the prompt request
type PromptAPIParameter struct {
	Name     string      `json:"name,omitempty"`
	Slug     string      `json:"slug"`
	Type     string      `json:"type,omitempty"`
	Desc     string      `json:"desc,omitempty"`
	Default  interface{} `json:"default,omitempty"`
	Required bool        `json:"required"`
	Options  interface{} `json:"options,omitempty"`
	Regex    string      `json:"regex,omitempty"`
}

// Config types and file uploads aren't supported in prompts yet.
func isUnsupportedParamType(t libapi.Type) bool {
	return t == libapi.TypeConfigVar || t == libapi.TypeUpload
}

// runbookParamsToPromptParams helps construct a map of prompt parameters
// in the valid format of {slug: PromptAPIParameterJSONObj}
func runbookParamsToPromptParams(params libapi.Parameters, runbookInfo runbookInfo) (interface{}, error) {
	o := ojson.NewObject()
	for _, p := range params {
		if isUnsupportedParamType(p.Type) {
			logger.Warning("- Skipping unsupported parameter - %s parameter types are not supported: %s.", p.Type, p.Slug)
			continue
		}
		defaultV, err := interfaceToJSObj(p.Default, nil, runbookInfo)
		if err != nil {
			return nil, err
		}

		formParam := PromptAPIParameter{
			Name:     p.Name,
			Slug:     p.Slug,
			Desc:     p.Desc,
			Type:     getParamType(p.Type, p.Component),
			Default:  defaultV,
			Required: !p.Constraints.Optional,
			Regex:    p.Constraints.Regex,
		}
		if len(p.Constraints.Options) > 0 {
			options, err := interfaceToJSObj(p.Constraints.Options, nil, runbookInfo)
			if err != nil {
				return nil, err
			}

			// in the case that the options are a dropdown (and not ConstraintOptions), manually construct them
			if _, ok := options.(templateValue); ok {
				options = []interface{}{map[string]interface{}{"label": "", "value": options}}
			}
			if options != nil {
				formParam.Options = options
			}
		}

		o.Set(p.Slug, formParam)
	}
	return o, nil
}

func (r *RunbookConverter) blockToString(
	ctx context.Context,
	block api.SessionBlock,
	runbookInfo runbookInfo,
	resources map[string]libapi.Resource,
) (string, error) {
	startCondition := transformTemplate(block.StartCondition, runbookInfo)

	if block.BlockKindConfig.Task != nil {
		config := block.BlockKindConfig.Task
		task, err := r.client.GetTaskByID(ctx, config.TaskID)
		if err != nil {
			return "", err
		}
		taskSlug := task.Slug

		taskParamValues, err := interfaceToJSObj(config.ParamValues, nil, runbookInfo)
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
			StartCondition: startCondition,
			TaskSlug:       taskSlug,
		})
		if err != nil {
			return "", err
		}

		return taskBlockStr, nil
	} else if block.BlockKindConfig.Note != nil {
		config := block.BlockKindConfig.Note

		noteContent, err := interfaceToJSObj(config.Content, nil, runbookInfo)
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
			StartCondition: startCondition,
		})
		if err != nil {
			return "", err
		}
		return noteBlockStr, nil
	} else if block.BlockKindConfig.Form != nil {
		config := block.BlockKindConfig.Form
		blockParams, err := runbookParamsToPromptParams(config.Parameters, runbookInfo)
		if err != nil {
			return "", err
		}
		formParams, err := interfaceToJSObj(blockParams, nil, runbookInfo)
		if err != nil {
			return "", err
		}
		formBlockStr, err := applyTemplate(formTemplate, struct {
			BlockSlug      string
			Params         interface{}
			StartCondition string
		}{
			BlockSlug:      block.Slug,
			Params:         formParams,
			StartCondition: startCondition,
		})
		if err != nil {
			return "", err
		}
		return formBlockStr, nil
	} else if block.BlockKindConfig.StdAPI != nil {
		config := block.BlockKindConfig.StdAPI

		req, ok := config.Request.(map[string]interface{})
		if !ok {
			return "", errors.Errorf("stdapi request was not map: %+v", config.Request)
		}

		switch config.Namespace {
		case "email":
			sender, err := interfaceToJSObj(req["sender"], nil, runbookInfo)
			if err != nil {
				return "", err
			}
			recipients, err := interfaceToJSObj(req["recipients"], nil, runbookInfo)
			if err != nil {
				return "", err
			}
			options, err := interfaceToJSObj(
				req, map[string]struct{}{
					"sender":     {},
					"recipients": {},
				},
				runbookInfo,
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
				Recipients:     recipients,
				ResourceSlug:   resourceSlug,
				Sender:         sender,
				StartCondition: startCondition,
			})
			if err != nil {
				return "", err
			}
			return emailBlockStr, nil
		case "graphql":
			operation, err := interfaceToJSObj(req["operation"], nil, runbookInfo)
			if err != nil {
				return "", err
			}

			options, err := interfaceToJSObj(
				req, map[string]struct{}{
					"operation": {},
				},
				runbookInfo,
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
				StartCondition: startCondition,
			})
			if err != nil {
				return "", err
			}
			return graphQLBlockStr, nil
		case "mongodb":
			collection, err := interfaceToJSObj(req["collection"], nil, runbookInfo)
			if err != nil {
				return "", err
			}

			options, err := interfaceToJSObj(
				req, map[string]struct{}{
					"collection": {},
				},
				runbookInfo,
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
				StartCondition: startCondition,
			})
			if err != nil {
				return "", err
			}
			return mongoDBBlockStr, nil
		case "rest":
			method, err := interfaceToJSObj(req["method"], nil, runbookInfo)
			if err != nil {
				return "", err
			}
			path, err := interfaceToJSObj(req["path"], nil, runbookInfo)
			if err != nil {
				return "", err
			}

			options, err := interfaceToJSObj(
				req, map[string]struct{}{
					"method": {},
					"path":   {},
				},
				runbookInfo,
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
				StartCondition: startCondition,
			})
			if err != nil {
				return "", err
			}
			return restBlockStr, nil
		case "slack":
			channel, err := interfaceToJSObj(req["channelName"], nil, runbookInfo)
			if err != nil {
				return "", err
			}
			message, err := interfaceToJSObj(req["message"], nil, runbookInfo)
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
				StartCondition: startCondition,
			})
			if err != nil {
				return "", err
			}
			return slackBlockStr, nil
		case "sql":
			query, err := interfaceToJSObj(req["query"], nil, runbookInfo)
			if err != nil {
				return "", err
			}

			queryArgs, err := interfaceToJSObj(req["queryArgs"], nil, runbookInfo)
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
				StartCondition: startCondition,
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
