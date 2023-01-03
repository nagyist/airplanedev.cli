package apiext

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/configs"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/params"
	"github.com/airplanedev/cli/pkg/print"
	"github.com/airplanedev/cli/pkg/resources"
	"github.com/airplanedev/cli/pkg/server/handlers"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/builtins"
	"github.com/airplanedev/ojson"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// AttachExternalAPIRoutes attaches a minimal subset of the actual Airplane API endpoints that are necessary to locally develop
// a task. For example, a workflow task might call airplane.execute, which would normally make a request to the
// /v0/tasks/execute endpoint in production, but instead we have our own implementation below.
func AttachExternalAPIRoutes(r *mux.Router, state *state.State) {
	const basePath = "/v0/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/tasks/execute", handlers.HandlerWithBody(state, ExecuteTaskHandler)).Methods("POST", "OPTIONS")
	r.Handle("/tasks/getMetadata", handlers.Handler(state, GetTaskMetadataHandler)).Methods("GET", "OPTIONS")
	r.Handle("/tasks/getTaskReviewers", handlers.Handler(state, GetTaskReviewersHandler)).Methods("GET", "OPTIONS")

	r.Handle("/runs/getOutputs", handlers.Handler(state, GetOutputsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/get", handlers.Handler(state, GetRunHandler)).Methods("GET", "OPTIONS")

	r.Handle("/resources/list", handlers.Handler(state, ListResourcesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/resources/listMetadata", handlers.Handler(state, ListResourceMetadataHandler)).Methods("GET", "OPTIONS")

	r.Handle("/views/get", handlers.Handler(state, GetViewHandler)).Methods("GET", "OPTIONS")

	r.Handle("/displays/create", handlers.HandlerWithBody(state, CreateDisplayHandler)).Methods("POST", "OPTIONS")

	r.Handle("/prompts/get", handlers.Handler(state, GetPromptHandler)).Methods("GET", "OPTIONS")
	r.Handle("/prompts/create", handlers.HandlerWithBody(state, CreatePromptHandler)).Methods("POST", "OPTIONS")

	// Run sleeps
	r.Handle("/sleeps/create", handlers.HandlerWithBody(state, CreateSleepHandler)).Methods("POST", "OPTIONS")
	r.Handle("/sleeps/list", handlers.Handler(state, ListSleepsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/sleeps/get", handlers.Handler(state, GetSleepHandler)).Methods("GET", "OPTIONS")

	r.Handle("/permissions/get", handlers.Handler(state, GetPermissionsHandler)).Methods("GET", "OPTIONS")

	r.Handle("/hosts/web", handlers.Handler(state, WebHostHandler)).Methods("GET", "OPTIONS")

	r.Handle("/uploads/create", handlers.HandlerWithBody(state, CreateUploadHandler)).Methods("POST", "OPTIONS")
}

type ExecuteTaskRequest struct {
	Slug        string            `json:"slug"`
	ParamValues api.Values        `json:"paramValues"`
	Resources   map[string]string `json:"resources"`
}

func getRunIDFromToken(r *http.Request) (string, error) {
	if token := r.Header.Get("X-Airplane-Token"); token != "" {
		claims, err := dev.ParseInsecureAirplaneToken(token)
		if err != nil {
			return "", err
		}
		return claims.RunID, nil
	}
	return "", nil
}

// ExecuteTaskHandler handles requests to the /v0/tasks/execute endpoint
func ExecuteTaskHandler(ctx context.Context, state *state.State, r *http.Request, req ExecuteTaskRequest) (api.RunTaskResponse, error) {
	run := *dev.NewLocalRun()
	parentID, err := getRunIDFromToken(r)
	if err != nil {
		return api.RunTaskResponse{}, err
	}
	run.ParentID = parentID

	runID := dev.GenerateRunID()
	run.ID = runID
	run.RunID = runID

	localTaskConfig, ok := state.TaskConfigs.Get(req.Slug)
	isBuiltin := builtins.IsBuiltinTaskSlug(req.Slug)
	parameters := libapi.Parameters{}
	start := time.Now().UTC()
	if isBuiltin || ok {
		runConfig := dev.LocalRunConfig{
			ID:             runID,
			ParamValues:    req.ParamValues,
			LocalClient:    state.LocalClient,
			RemoteClient:   state.RemoteClient,
			UseFallbackEnv: state.UseFallbackEnv,
			Slug:           req.Slug,
			ParentRunID:    pointers.String(parentID),
			IsBuiltin:      isBuiltin,
			AuthInfo:       state.AuthInfo,
			LogBroker:      run.LogBroker,
			WorkingDir:     state.Dir,
			StudioURL:      state.StudioURL,
		}
		resourceAttachments := map[string]string{}
		mergedResources, err := resources.MergeRemoteResources(ctx, state)
		if err != nil {
			return api.RunTaskResponse{}, errors.Wrap(err, "merging local and remote resources")
		}
		// Builtins have a specific alias in the form of "rest", "db", etc. that is required by the builtins binary,
		// and so we need to manually generate resource attachments.
		if isBuiltin {
			// The SDK should provide us with exactly one resource for builtins.
			if len(req.Resources) != 1 {
				return api.RunTaskResponse{}, errors.Errorf("unable to determine resource required by builtin, there is not exactly one resource in request: %+v", req.Resources)
			}

			// Get the only entry in the request resource map.
			var builtinAlias, resourceID string
			for builtinAlias, resourceID = range req.Resources {
			}

			var foundResource bool
			for slug, res := range mergedResources {
				if res.Resource.GetID() == resourceID {
					resourceAttachments[builtinAlias] = slug
					foundResource = true
				}
			}

			if !foundResource {
				if resourceID == resources.SlackID {
					return api.RunTaskResponse{}, errors.New("Your team has not configured Slack. Please visit https://docs.airplane.dev/platform/slack-integration#connect-to-slack to authorize Slack to perform actions in your workspace.")
				}
				return api.RunTaskResponse{}, errors.Errorf("Resource with id %s not found in dev config file or remotely.", resourceID)
			}
			run.IsStdAPI = true
			stdapiReq, err := builtins.Request(req.Slug, req.ParamValues)
			if err != nil {
				return api.RunTaskResponse{}, err
			}
			run.StdAPIRequest = stdapiReq
			run.TaskName = req.Slug
			run.ParamValues = req.ParamValues
		} else if localTaskConfig.Def != nil {
			kind, kindOptions, err := dev.GetKindAndOptions(localTaskConfig)
			if err != nil {
				return api.RunTaskResponse{}, err
			}
			runConfig.Kind = kind
			runConfig.KindOptions = kindOptions
			runConfig.Name = localTaskConfig.Def.GetName()
			runConfig.File = localTaskConfig.TaskEntrypoint
			resourceAttachments, err = localTaskConfig.Def.GetResourceAttachments()
			if err != nil {
				return api.RunTaskResponse{}, errors.Wrap(err, "getting resource attachments")
			}
			if runConfig.EnvVars, err = localTaskConfig.Def.GetEnv(); err != nil {
				return api.RunTaskResponse{}, errors.Wrap(err, "getting task env vars")
			}
			if runConfig.ConfigAttachments, err = localTaskConfig.Def.GetConfigAttachments(); err != nil {
				return api.RunTaskResponse{}, errors.Wrap(err, "getting attached configs")
			}
			parameters, err = localTaskConfig.Def.GetParameters()
			if err != nil {
				return api.RunTaskResponse{}, errors.Wrap(err, "getting parameters")
			}
			run.TaskID = req.Slug
			run.TaskName = localTaskConfig.Def.GetName()
			runConfig.ConfigVars, err = configs.MergeRemoteConfigs(ctx, state)
			if err != nil {
				return api.RunTaskResponse{}, errors.Wrap(err, "merging local and remote configs")
			}
			run.TaskRevision = localTaskConfig
			paramValuesWithDefaults := params.ApplyDefaults(parameters, req.ParamValues)
			run.ParamValues = paramValuesWithDefaults
			runConfig.ParamValues, err = params.StandardizeParamValues(ctx, state.RemoteClient, parameters, paramValuesWithDefaults)
			if err != nil {
				return api.RunTaskResponse{}, err
			}
		}
		aliasToResourceMap, err := resources.GenerateAliasToResourceMap(
			ctx,
			state,
			resourceAttachments,
			mergedResources,
		)
		if err != nil {
			return api.RunTaskResponse{}, errors.Wrap(err, "generating alias to resource map")
		}
		runConfig.AliasToResource = aliasToResourceMap
		run.Resources = resources.GenerateResourceAliasToID(aliasToResourceMap)
		run.CreatedAt = start

		run.Parameters = &parameters

		run.Status = api.RunActive
		runCtx, fn := context.WithCancel(context.Background()) // Context used for cancelling a run.
		run.CancelFn = fn
		// if the user is authenticated in CLI, use their ID
		if state.AuthInfo.User != nil {
			run.CreatorID = state.AuthInfo.User.ID
		}
		state.Runs.Add(req.Slug, runID, run)

		// Use a new context while executing so the handler context doesn't cancel task execution
		go func() {
			outputs, err := state.Executor.Execute(runCtx, runConfig)
			completedAt := time.Now()

			status := api.RunSucceeded
			var succeededAt *time.Time
			var failedAt *time.Time

			if err == nil {
				succeededAt = &completedAt
			} else {
				runState, _ := state.Runs.Get(runID)
				if runState.Status == api.RunCancelled {
					status = api.RunCancelled
				} else {
					status = api.RunFailed
					failedAt = &completedAt
					// If an error output isn't already set, set it here.
					if outputs.V == nil {
						outputs = api.Outputs{

							V: ojson.NewObject().SetAndReturn("error", err.Error()),
						}
					}
				}
			}

			if _, err = state.Runs.Update(runID, func(run *dev.LocalRun) error {
				run.Outputs = outputs
				run.Status = status
				run.SucceededAt = succeededAt
				run.FailedAt = failedAt
				return nil
			}); err != nil {
				logger.Error("updating run with status: %+v", err)
			}
		}()
	} else {
		if !state.UseFallbackEnv {
			return api.RunTaskResponse{}, errors.Errorf("task with slug %s is not registered locally", req.Slug)
		}

		resp, err := state.RemoteClient.RunTask(ctx, api.RunTaskRequest{
			TaskSlug:    &req.Slug,
			ParamValues: req.ParamValues,
			EnvSlug:     state.RemoteEnv.Slug,
		})
		if err != nil {
			var taskMissingError *libapi.TaskMissingError
			if errors.As(err, taskMissingError) {
				return api.RunTaskResponse{}, errors.Errorf("task with slug %s is not registered locally or remotely", req.Slug)
			} else {
				return api.RunTaskResponse{}, err
			}
		}

		run.Remote = true
		run.ID = resp.RunID
		run.RunID = resp.RunID
		state.Runs.Add(req.Slug, resp.RunID, run)
		return api.RunTaskResponse{RunID: resp.RunID}, nil
	}

	return api.RunTaskResponse{RunID: runID}, nil
}

// GetRunHandler handles requests to the /v0/runs/get endpoint
func GetRunHandler(ctx context.Context, state *state.State, r *http.Request) (dev.LocalRun, error) {
	runID := r.URL.Query().Get("id")
	run, ok := state.Runs.Get(runID)
	if !ok {
		return dev.LocalRun{}, errors.Errorf("run with id %s not found", runID)
	}

	if run.Remote {
		resp, err := state.RemoteClient.GetRun(ctx, runID)
		if err != nil {
			return dev.LocalRun{}, errors.Wrap(err, "getting remote run")
		}
		remoteRun := resp.Run

		return dev.LocalRun{
			ID:          runID,
			RunID:       runID,
			Status:      remoteRun.Status,
			CreatedAt:   remoteRun.CreatedAt,
			CreatorID:   remoteRun.CreatorID,
			SucceededAt: remoteRun.SucceededAt,
			FailedAt:    remoteRun.FailedAt,
			ParamValues: remoteRun.ParamValues,
			TaskID:      remoteRun.TaskID,
			TaskName:    remoteRun.TaskName,
			Remote:      true,
		}, nil
	}

	return run, nil
}

// GetTaskMetadataHandler handles requests to the /v0/tasks/getMetadata endpoint. It generates a deterministic task ID
// for each task found locally, and its primary purpose is to ensure that the task discoverer does not error.
// If a task is not local, it tries the fallback environment, so that local views
// can route correctly to the correct URL.
func GetTaskMetadataHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.TaskMetadata, error) {
	slug := r.URL.Query().Get("slug")
	_, ok := state.TaskConfigs.Get(slug)
	isBuiltin := builtins.IsBuiltinTaskSlug(slug)
	// Neither builtin nor local, we try using the fallback env first, but we
	// default to returning a dummy task if it's not found.
	if !isBuiltin && !ok {
		if state.UseFallbackEnv {
			resp, err := state.RemoteClient.GetTaskMetadata(ctx, slug)
			if err != nil {
				logger.Debug("Received error %s from remote task metadata, falling back to default", err)
			} else {
				return resp, nil
			}
		}
	}
	return libapi.TaskMetadata{
		ID:      fmt.Sprintf("tsk-%s", slug),
		Slug:    slug,
		IsLocal: true,
	}, nil
}

func GetTaskReviewersHandler(ctx context.Context, state *state.State, r *http.Request) (api.GetTaskReviewersResponse, error) {
	taskSlug := r.URL.Query().Get("taskSlug")
	localTaskConfig, ok := state.TaskConfigs.Get(taskSlug)
	if ok {
		parameters, err := localTaskConfig.Def.GetParameters()
		if err != nil {
			return api.GetTaskReviewersResponse{}, err
		}
		return api.GetTaskReviewersResponse{
			Task: &libapi.Task{
				Slug:       taskSlug,
				Parameters: parameters,
				Triggers:   []libapi.Trigger{{Kind: "form"}},
			},
		}, nil
	}
	if !state.UseFallbackEnv {
		return api.GetTaskReviewersResponse{}, errors.Errorf("task with slug %s is not registered locally", taskSlug)
	}

	resp, err := state.RemoteClient.GetTaskReviewers(ctx, taskSlug)
	if err != nil {
		var taskMissingError *libapi.TaskMissingError
		if errors.As(err, taskMissingError) {
			return api.GetTaskReviewersResponse{}, errors.Errorf("task with slug %s is not registered locally or remotely", taskSlug)
		} else {
			return api.GetTaskReviewersResponse{}, err
		}
	}
	return resp, nil
}

// GetViewHandler handles requests to the /v0/views/get endpoint. It generates a deterministic view ID for each view
// found locally, and its primary purpose is to ensure that the view discoverer does not error.
// If a view is not local, it tries the fallback environment, so that local views
// can route correctly to the correct URL.
func GetViewHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.View, error) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		return libapi.View{}, errors.New("slug cannot be empty")
	}

	_, ok := state.ViewConfigs.Get(slug)
	// Not local, we try using the fallback env first, but we default to returning a dummy view if it's not found.
	if !ok {
		if state.UseFallbackEnv {
			resp, err := state.RemoteClient.GetView(ctx, libapi.GetViewRequest{
				Slug: slug,
			})
			if err != nil {
				logger.Debug("Received error %s from remote view, falling back to default", err)
			} else {
				return resp, nil
			}
		}
	}

	return libapi.View{
		ID:      fmt.Sprintf("vew-%s", slug),
		Slug:    r.URL.Query().Get("slug"),
		IsLocal: true,
	}, nil
}

type ListRunsResponse struct {
	Runs []dev.LocalRun `json:"runs"`
}

func ListRunsHandler(ctx context.Context, state *state.State, r *http.Request) (ListRunsResponse, error) {
	taskSlug := r.URL.Query().Get("taskSlug")
	runs := state.Runs.GetRunHistory(taskSlug)
	return ListRunsResponse{
		Runs: runs,
	}, nil
}

type GetOutputsResponse struct {
	// Outputs from this run.
	Output api.Outputs `json:"output"`
}

// GetOutputsHandler handles requests to the /v0/runs/getOutputs endpoint
func GetOutputsHandler(ctx context.Context, state *state.State, r *http.Request) (GetOutputsResponse, error) {
	runID := r.URL.Query().Get("id")
	run, ok := state.Runs.Get(runID)
	if !ok {
		return GetOutputsResponse{}, errors.Errorf("run with id %s not found", runID)
	}
	outputs := run.Outputs

	if run.Remote {
		resp, err := state.RemoteClient.GetOutputs(ctx, runID)
		if err != nil {
			return GetOutputsResponse{}, errors.Wrap(err, "getting remote run")
		}

		outputs = resp.Outputs
	}

	return GetOutputsResponse{
		Output: outputs,
	}, nil
}

type CreateDisplayRequest struct {
	Display libapi.Display `json:"display"`
}

type CreateDisplayResponse struct {
	Display libapi.Display `json:"display"`
}

func CreateDisplayHandler(ctx context.Context, state *state.State, r *http.Request, req CreateDisplayRequest) (CreateDisplayResponse, error) {
	token := r.Header.Get("X-Airplane-Token")
	if token == "" {
		return CreateDisplayResponse{}, errors.Errorf("expected a X-Airplane-Token header")
	}
	claims, err := dev.ParseInsecureAirplaneToken(token)
	if err != nil {
		return CreateDisplayResponse{}, errors.Errorf("invalid airplane token: %s", err.Error())
	}
	runID := claims.RunID

	now := time.Now()
	display := libapi.Display{
		ID:        utils.GenerateID("dsp"),
		RunID:     runID,
		Kind:      req.Display.Kind,
		CreatedAt: now,
		UpdatedAt: now,
	}
	switch req.Display.Kind {
	case "markdown":
		display.Content = req.Display.Content

		maxMarkdownLength := 100000
		if len(display.Content) > maxMarkdownLength {
			return CreateDisplayResponse{}, errors.Errorf("content too long: expected at most %d characters, got %d", maxMarkdownLength, len(display.Content))
		}
	case "table":
		display.Rows = req.Display.Rows
		display.Columns = req.Display.Columns

		maxRows := 10000
		if len(display.Rows) > maxRows {
			return CreateDisplayResponse{}, errors.Errorf("too many table rows: expected at most %d, got %d", maxRows, len(display.Rows))
		}

		maxColumns := 100
		if len(display.Columns) > maxColumns {
			return CreateDisplayResponse{}, errors.Errorf("too many table columns: expected at most %d, got %d", maxColumns, len(display.Columns))
		}
	case "json":
		display.Value = req.Display.Value
	}

	run, err := state.Runs.Update(runID, func(run *dev.LocalRun) error {
		run.Displays = append(run.Displays, display)
		return nil
	})
	if err != nil {
		return CreateDisplayResponse{}, err
	}

	content := fmt.Sprintf("[kind=%s]\n\n%s", display.Kind, display.Content)
	prefix := "[" + logger.Gray(run.TaskID+" display") + "] "
	print.BoxPrintWithPrefix(content, prefix)

	return CreateDisplayResponse{
		Display: display,
	}, nil
}

type PromptResponse struct {
	ID string `json:"id"`
}

func CreatePromptHandler(ctx context.Context, state *state.State, r *http.Request, req libapi.Prompt) (PromptResponse, error) {
	runID, err := getRunIDFromToken(r)
	if err != nil {
		return PromptResponse{}, err
	}
	if runID == "" {
		return PromptResponse{}, errors.Errorf("expected runID from airplane token: %s", err.Error())
	}

	if req.Values == nil {
		req.Values = map[string]interface{}{}
	}

	reviewersWithDefaults := req.Reviewers
	if reviewersWithDefaults == nil {
		reviewersWithDefaults = &libapi.PromptReviewers{}
	}
	if reviewersWithDefaults.AllowSelfApprovals == nil {
		// Default self approvals to true
		reviewersWithDefaults.AllowSelfApprovals = pointers.Bool(true)
	}
	if reviewersWithDefaults.Groups == nil {
		reviewersWithDefaults.Groups = []string{}
	}
	if reviewersWithDefaults.Users == nil {
		reviewersWithDefaults.Users = []string{}
	}

	prompt := libapi.Prompt{
		ID:          utils.GenerateID("pmt"),
		RunID:       runID,
		Schema:      req.Schema,
		Values:      req.Values,
		Reviewers:   reviewersWithDefaults,
		ConfirmText: req.ConfirmText,
		CancelText:  req.CancelText,
		CreatedAt:   time.Now(),
		Description: req.Description,
	}

	if _, err := state.Runs.Update(runID, func(run *dev.LocalRun) error {
		run.Prompts = append(run.Prompts, prompt)
		run.IsWaitingForUser = true
		return nil
	}); err != nil {
		return PromptResponse{}, err
	}

	return PromptResponse{ID: prompt.ID}, nil
}

type GetPromptResponse struct {
	Prompt libapi.Prompt `json:"prompt"`
}

func GetPromptHandler(ctx context.Context, state *state.State, r *http.Request) (GetPromptResponse, error) {
	promptID := r.URL.Query().Get("id")
	if promptID == "" {
		return GetPromptResponse{}, errors.New("id is required")
	}
	runID, err := getRunIDFromToken(r)
	if err != nil {
		return GetPromptResponse{}, err
	}
	if runID == "" {
		return GetPromptResponse{}, errors.Errorf("expected runID from airplane token")
	}

	run, ok := state.Runs.Get(runID)
	if !ok {
		return GetPromptResponse{}, errors.New("run not found")
	}

	for _, p := range run.Prompts {
		if p.ID == promptID {
			return GetPromptResponse{Prompt: p}, nil
		}
	}
	return GetPromptResponse{}, errors.New("prompt not found")
}

// ListResourcesHandler handles requests to the /i/resources/list endpoint
func ListResourcesHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.ListResourcesResponse, error) {
	resources := make([]libapi.Resource, 0, len(state.DevConfig.RawResources))
	for slug, r := range state.DevConfig.Resources {
		resources = append(resources, libapi.Resource{
			ID:                r.Resource.GetID(),
			Slug:              slug,
			Kind:              libapi.ResourceKind(r.Resource.Kind()),
			ExportResource:    r.Resource,
			CanUseResource:    true,
			CanUpdateResource: true,
		})
	}

	return libapi.ListResourcesResponse{
		Resources: resources,
	}, nil
}

// ListResourceMetadataHandler handles requests to the /v0/resources/listMetadata endpoint
func ListResourceMetadataHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.ListResourceMetadataResponse, error) {
	mergedResources, err := resources.MergeRemoteResources(ctx, state)
	if err != nil {
		return libapi.ListResourceMetadataResponse{}, errors.Wrap(err, "merging local and remote resources")
	}

	resources := make([]libapi.ResourceMetadata, 0, len(mergedResources))
	for slug, resourceWithEnv := range mergedResources {
		res := resourceWithEnv.Resource
		resources = append(resources, libapi.ResourceMetadata{
			ID:   res.GetID(),
			Slug: slug,
			DefaultEnvResource: &libapi.Resource{
				ID:             res.GetID(),
				Name:           res.GetName(),
				Slug:           slug,
				Kind:           libapi.ResourceKind(res.Kind()),
				ExportResource: res,
			},
		})
	}

	return libapi.ListResourceMetadataResponse{
		Resources: resources,
	}, nil
}

func GetPermissionsHandler(ctx context.Context, state *state.State, r *http.Request) (api.GetPermissionsResponse, error) {
	taskSlug := r.URL.Query().Get("task_slug")
	actions := r.URL.Query()["actions"]
	_, hasLocalTask := state.TaskConfigs.Get(taskSlug)

	outputs := map[string]bool{}
	if hasLocalTask {
		for _, action := range actions {
			outputs[action] = true
		}
		return api.GetPermissionsResponse{
			Outputs: outputs,
		}, nil
	}

	return state.RemoteClient.GetPermissions(ctx, taskSlug, actions)
}

func WebHostHandler(ctx context.Context, state *state.State, r *http.Request) (string, error) {
	return state.RemoteClient.GetWebHost(ctx)
}

func CreateUploadHandler(
	ctx context.Context,
	state *state.State,
	r *http.Request,
	req libapi.CreateUploadRequest,
) (libapi.CreateUploadResponse, error) {
	resp, err := state.RemoteClient.CreateUpload(ctx, req)
	if err != nil {
		return libapi.CreateUploadResponse{}, errors.Wrap(err, "creating upload")
	}

	return libapi.CreateUploadResponse{
		Upload:       resp.Upload,
		ReadOnlyURL:  resp.ReadOnlyURL,
		WriteOnlyURL: resp.WriteOnlyURL,
	}, nil
}
