package apiext

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/configs"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/params"
	"github.com/airplanedev/cli/pkg/print"
	"github.com/airplanedev/cli/pkg/resources"
	"github.com/airplanedev/cli/pkg/server/handlers"
	"github.com/airplanedev/cli/pkg/server/outputs"
	"github.com/airplanedev/cli/pkg/server/state"
	serverutils "github.com/airplanedev/cli/pkg/server/utils"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	libapi "github.com/airplanedev/lib/pkg/api"
	libhttp "github.com/airplanedev/lib/pkg/api/http"
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

	r.Handle("/tasks/execute", handlers.WithBody(state, ExecuteTaskHandler)).Methods("POST", "OPTIONS")
	r.Handle("/tasks/getMetadata", handlers.New(state, GetTaskMetadataHandler)).Methods("GET", "OPTIONS")
	r.Handle("/tasks/getTaskReviewers", handlers.New(state, GetTaskReviewersHandler)).Methods("GET", "OPTIONS")

	r.Handle("/entities/search", handlers.New(state, SearchEntitiesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runners/createScaleSignal", handlers.WithBody(state, CreateScaleSignalHandler)).Methods("POST", "OPTIONS")

	r.Handle("/runs/getOutputs", handlers.New(state, outputs.GetOutputsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/get", handlers.New(state, GetRunHandler)).Methods("GET", "OPTIONS")

	r.Handle("/resources/list", handlers.New(state, ListResourcesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/resources/listMetadata", handlers.New(state, ListResourceMetadataHandler)).Methods("GET", "OPTIONS")

	r.Handle("/views/get", handlers.New(state, GetViewHandler)).Methods("GET", "OPTIONS")

	r.Handle("/displays/create", handlers.WithBody(state, CreateDisplayHandler)).Methods("POST", "OPTIONS")

	r.Handle("/prompts/get", handlers.New(state, GetPromptHandler)).Methods("GET", "OPTIONS")
	r.Handle("/prompts/create", handlers.WithBody(state, CreatePromptHandler)).Methods("POST", "OPTIONS")

	// Run sleeps
	r.Handle("/sleeps/create", handlers.WithBody(state, CreateSleepHandler)).Methods("POST", "OPTIONS")
	r.Handle("/sleeps/list", handlers.New(state, ListSleepsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/sleeps/get", handlers.New(state, GetSleepHandler)).Methods("GET", "OPTIONS")

	r.Handle("/permissions/get", handlers.New(state, GetPermissionsHandler)).Methods("GET", "OPTIONS")

	r.Handle("/hosts/web", handlers.New(state, WebHostHandler)).Methods("GET", "OPTIONS")

	r.Handle("/uploads/create", handlers.WithBody(state, CreateUploadHandler)).Methods("POST", "OPTIONS")
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

type ExecuteTaskRequest struct {
	Slug        string            `json:"slug"`
	ParamValues api.Values        `json:"paramValues"`
	Resources   map[string]string `json:"resources"`
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

	// Pull env slug from the parent run.
	var envSlug *string
	if parentID != "" {
		parentRun, ok := state.Runs.Get(parentID)
		if !ok {
			return api.RunTaskResponse{}, libhttp.NewErrNotFound("run with parent id %q not found", parentID)
		}
		if parentRun.FallbackEnvSlug != "" {
			envSlug = &parentRun.FallbackEnvSlug
		}
	} else {
		envSlug = serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	}

	localTaskConfig, ok := state.TaskConfigs.Get(req.Slug)
	isBuiltin := builtins.IsBuiltinTaskSlug(req.Slug)
	parameters := libapi.Parameters{}
	start := time.Now().UTC()
	if isBuiltin || ok {
		runConfig := dev.LocalRunConfig{
			ID:              runID,
			ParamValues:     req.ParamValues,
			LocalClient:     state.LocalClient,
			RemoteClient:    state.RemoteClient,
			TunnelToken:     state.DevToken,
			FallbackEnvSlug: pointers.ToString(envSlug),
			Slug:            req.Slug,
			ParentRunID:     pointers.String(parentID),
			IsBuiltin:       isBuiltin,
			AuthInfo:        state.AuthInfo,
			LogBroker:       run.LogBroker,
			WorkingDir:      state.Dir,
			StudioURL:       state.StudioURL,
			EnvVars:         state.DevConfig.EnvVars,
		}
		resourceAttachments := map[string]string{}
		mergedResources, err := resources.MergeRemoteResources(ctx, state.RemoteClient, state.DevConfig, envSlug)
		if err != nil {
			return api.RunTaskResponse{}, errors.Wrap(err, "merging local and remote resources")
		}
		// Builtins have a specific alias in the form of "rest", "db", etc. that is required by the builtins binary,
		// and so we need to manually generate resource attachments.
		if isBuiltin {
			// The SDK should provide us with exactly one resource for builtins.
			if len(req.Resources) != 1 {
				return api.RunTaskResponse{}, libhttp.NewErrBadRequest("unable to determine resource required by builtin, there is not exactly one resource in request: %+v", req.Resources)
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
				message := fmt.Sprintf("resource with id %q not found in dev config file or remotely", resourceID)
				if resourceID == resources.SlackID {
					message = "your team has not configured Slack. Please visit https://docs.airplane.dev/platform/slack-integration#connect-to-slack to authorize Slack to perform actions in your workspace."
				}
				return api.RunTaskResponse{}, libhttp.NewErrNotFound(message)
			}
			run.IsStdAPI = true
			stdapiReq, err := builtins.Request(req.Slug, req.ParamValues)
			if err != nil {
				return api.RunTaskResponse{}, err
			}
			run.StdAPIRequest = stdapiReq
			run.TaskName = req.Slug
			run.ParamValues = req.ParamValues
		} else {
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
			if runConfig.TaskEnvVars, err = localTaskConfig.Def.GetEnv(); err != nil {
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
			runConfig.ConfigVars, err = configs.MergeRemoteConfigs(ctx, state, envSlug)
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
			resourceAttachments,
			mergedResources,
			envSlug,
			state.RemoteClient,
		)
		if err != nil {
			return api.RunTaskResponse{}, err
		}
		runConfig.AliasToResource = aliasToResourceMap
		run.Resources = resources.GenerateResourceAliasToID(aliasToResourceMap)
		run.CreatedAt = start

		run.Parameters = &parameters
		run.FallbackEnvSlug = pointers.ToString(envSlug)

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

				// If the process was killed by a signal, the builtins binary is likely corrupt. Manually trigger a
				// re-download of the builtins binary.
				exitErr := &exec.ExitError{}
				if errors.As(err, &exitErr) && exitErr.ExitCode() == -1 { // -1 is the exit code for killed processes
					if err := state.Executor.Refresh(); err != nil {
						logger.Debug("refreshing executor: %+v", err)
					}

					outputs = api.Outputs{
						V: ojson.NewObject().SetAndReturn(
							"error",
							fmt.Sprintf(
								"We detected some corrupted files in your %s directory. We've reinitialized this directory for you, please try executing the task again.",
								filepath.Join(filepath.Base(state.Dir), ".airplane"),
							),
						),
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
		if envSlug == nil {
			return api.RunTaskResponse{}, libhttp.NewErrNotFound("task with slug %q is not registered locally", req.Slug)
		}

		resp, err := state.RemoteClient.RunTask(ctx, api.RunTaskRequest{
			TaskSlug:    &req.Slug,
			ParamValues: req.ParamValues,
			EnvSlug:     *envSlug,
		})
		if err != nil {
			var taskMissingError *libapi.TaskMissingError
			if errors.As(err, &taskMissingError) {
				return api.RunTaskResponse{}, libhttp.NewErrNotFound("task with slug %q is not registered locally or remotely", req.Slug)
			} else {
				return api.RunTaskResponse{}, err
			}
		}

		run.Remote = true
		run.ID = resp.RunID
		run.RunID = resp.RunID
		run.EnvSlug = pointers.ToString(envSlug)
		run.FallbackEnvSlug = pointers.ToString(envSlug)
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
		return dev.LocalRun{}, libhttp.NewErrNotFound("run with id %q not found", runID)
	}

	if run.Remote {
		resp, err := state.RemoteClient.GetRun(ctx, runID)
		if err != nil {
			return dev.LocalRun{}, errors.Wrap(err, "getting remote run")
		}
		return dev.FromRemoteRun(resp.Run), nil
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
		if state.InitialRemoteEnvSlug != nil {
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
	if state.InitialRemoteEnvSlug == nil {
		return api.GetTaskReviewersResponse{}, libhttp.NewErrNotFound("task with slug %q is not registered locally", taskSlug)
	}

	resp, err := state.RemoteClient.GetTaskReviewers(ctx, taskSlug)
	if err != nil {
		var taskMissingError *libapi.TaskMissingError
		if errors.As(err, &taskMissingError) {
			return api.GetTaskReviewersResponse{}, libhttp.NewErrNotFound("task with slug %q is not registered locally or remotely", taskSlug)
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
		return libapi.View{}, libhttp.NewErrBadRequest("slug cannot be empty")
	}

	_, ok := state.ViewConfigs.Get(slug)
	// Not local, we try using the fallback env first, but we default to returning a dummy view if it's not found.
	if !ok {
		if state.InitialRemoteEnvSlug != nil {
			// TODO: should probably pass env into GetView
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

type CreateDisplayRequest struct {
	Display libapi.Display `json:"display"`
}

type CreateDisplayResponse struct {
	ID string `json:"id"`
}

func CreateDisplayHandler(ctx context.Context, state *state.State, r *http.Request, req CreateDisplayRequest) (CreateDisplayResponse, error) {
	token := r.Header.Get("X-Airplane-Token")
	if token == "" {
		return CreateDisplayResponse{}, libhttp.NewErrBadRequest("expected a X-Airplane-Token header")
	}
	claims, err := dev.ParseInsecureAirplaneToken(token)
	if err != nil {
		return CreateDisplayResponse{}, libhttp.NewErrBadRequest("invalid airplane token: %s", err.Error())
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
			return CreateDisplayResponse{}, libhttp.NewErrBadRequest("content too long: expected at most %d characters, got %d", maxMarkdownLength, len(display.Content))
		}
	case "table":
		display.Rows = req.Display.Rows
		display.Columns = req.Display.Columns

		maxRows := 10000
		if len(display.Rows) > maxRows {
			return CreateDisplayResponse{}, libhttp.NewErrBadRequest("too many table rows: expected at most %d, got %d", maxRows, len(display.Rows))
		}

		maxColumns := 100
		if len(display.Columns) > maxColumns {
			return CreateDisplayResponse{}, libhttp.NewErrBadRequest("too many table columns: expected at most %d, got %d", maxColumns, len(display.Columns))
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
		ID: display.ID,
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
		return PromptResponse{}, libhttp.NewErrBadRequest("expected runID from airplane token: %s", err.Error())
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
		return GetPromptResponse{}, libhttp.NewErrBadRequest("id is required")
	}
	runID, err := getRunIDFromToken(r)
	if err != nil {
		return GetPromptResponse{}, err
	}
	if runID == "" {
		return GetPromptResponse{}, libhttp.NewErrBadRequest("expected runID from airplane token")
	}

	run, ok := state.Runs.Get(runID)
	if !ok {
		return GetPromptResponse{}, libhttp.NewErrNotFound("run not found")
	}

	for _, p := range run.Prompts {
		if p.ID == promptID {
			return GetPromptResponse{Prompt: p}, nil
		}
	}
	return GetPromptResponse{}, libhttp.NewErrNotFound("prompt not found")
}

// ListResourcesHandler handles requests to the /v0/resources/list endpoint
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
	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	mergedResources, err := resources.MergeRemoteResources(ctx, state.RemoteClient, state.DevConfig, envSlug)
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

type CreateRunnerScaleSignalRequest struct {
	SignalKey                 string  `json:"signalKey"`
	ExpirationDurationSeconds int     `json:"expirationDurationSeconds"`
	TaskSlug                  *string `json:"taskSlug"`
	TaskID                    *string `json:"taskID"`
	IsStdAPI                  *bool   `json:"isStdAPI"`
	TaskRevisionID            *string `json:"taskRevisionID"`
}

type CreateRunnerScaleSignalResponse struct{}

func CreateScaleSignalHandler(
	ctx context.Context,
	state *state.State,
	r *http.Request,
	req CreateRunnerScaleSignalRequest,
) (CreateRunnerScaleSignalResponse, error) {
	return CreateRunnerScaleSignalResponse{}, nil
}

type SearchEntitiesResponse struct {
	Results []struct{} `json:"results"`
}

func SearchEntitiesHandler(
	ctx context.Context,
	state *state.State,
	r *http.Request,
) (SearchEntitiesResponse, error) {
	return SearchEntitiesResponse{
		Results: []struct{}{},
	}, nil
}
