package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/resource"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/builtins"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

type LocalRun struct {
	RunID       string                 `json:"runID"`
	Status      api.RunStatus          `json:"status"`
	Outputs     api.Outputs            `json:"outputs"`
	CreatedAt   time.Time              `json:"createdAt"`
	CreatorID   string                 `json:"creatorID"`
	SucceededAt *time.Time             `json:"succeededAt"`
	FailedAt    *time.Time             `json:"failedAt"`
	ParamValues map[string]interface{} `json:"paramValues"`
	Parameters  *libapi.Parameters     `json:"parameters"`
	LogStore    *dev.LogStore          `json:"-"`
	TaskName    string                 `json:"taskName"`
}

// NewLocalRun initializes a run for local dev.
func NewLocalRun() *LocalRun {
	return &LocalRun{
		Status:      api.RunQueued,
		ParamValues: map[string]interface{}{},
		CreatedAt:   time.Now(),
		LogStore: &dev.LogStore{
			Channel:     make(chan dev.ResponseLog),
			DoneChannel: make(chan bool, 1),
			Logs:        make([]dev.ResponseLog, 0),
		},
	}
}

// AttachAPIRoutes attaches a minimal subset of the actual Airplane API endpoints that are necessary to locally develop
// a task. For example, a workflow task might call airplane.execute, which would normally make a request to the
// /v0/tasks/execute endpoint in production, but instead we have our own implementation below.
func AttachAPIRoutes(r *mux.Router, state *State) {
	const basePath = "/v0/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/tasks/execute", HandlerWithBody(state, ExecuteTaskHandler)).Methods("POST", "OPTIONS")
	r.Handle("/tasks/getMetadata", Handler(state, GetTaskMetadataHandler)).Methods("GET", "OPTIONS")
	r.Handle("/tasks/get", Handler(state, GetTaskInfoHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/getOutputs", Handler(state, GetOutputsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/get", Handler(state, GetRunHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/list", Handler(state, ListRunsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/resources/list", Handler(state, ListResourcesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/views/get", Handler(state, GetViewHandler)).Methods("GET", "OPTIONS")
}

type ExecuteTaskRequest struct {
	RunID       string            `json:"runID"`
	Slug        string            `json:"slug"`
	ParamValues api.Values        `json:"paramValues"`
	Resources   map[string]string `json:"resources"`
}

// ExecuteTaskHandler handles requests to the /v0/tasks/execute endpoint
func ExecuteTaskHandler(ctx context.Context, state *State, req ExecuteTaskRequest) (LocalRun, error) {
	// Allow run IDs to be generated beforehand; this is needed so that the /dev/logs endpoint can start waiting
	// for logs for a given run before that run's execution has started.
	runID := req.RunID
	var run LocalRun
	if runID != "" {
		run, _ = state.runs.get(runID)
	} else {
		runID = "run" + utils.RandomString(10, utils.CharsetLowercaseNumeric)
		run = *NewLocalRun()
	}

	localTaskConfig, ok := state.taskConfigs[req.Slug]
	isBuiltin := builtins.IsBuiltinTaskSlug(req.Slug)
	var parameters libapi.Parameters
	start := time.Now()
	if isBuiltin || ok {
		runConfig := dev.LocalRunConfig{
			ParamValues: req.ParamValues,
			Port:        state.port,
			Root:        state.cliConfig,
			Slug:        req.Slug,
			EnvSlug:     state.envSlug,
			IsBuiltin:   isBuiltin,
			LogStore:    run.LogStore,
		}
		resourceAttachments := map[string]string{}
		if isBuiltin {
			// builtins have access to all resources the parent task has
			for slug := range state.devConfig.DecodedResources {
				resourceAttachments[slug] = slug
			}
		} else if localTaskConfig.Def != nil {
			kind, kindOptions, err := localTaskConfig.Def.GetKindAndOptions()
			if err != nil {
				return LocalRun{}, errors.Wrap(err, "failed to get kind and options from task config")
			}
			runConfig.Kind = kind
			runConfig.KindOptions = kindOptions
			runConfig.Name = localTaskConfig.Def.GetName()
			runConfig.File = localTaskConfig.Def.GetDefnFilePath()
			resourceAttachments = localTaskConfig.Def.GetResourceAttachments()
			parameters = localTaskConfig.Def.GetParameters()

		}
		resources, err := resource.GenerateAliasToResourceMap(
			resourceAttachments,
			state.devConfig.DecodedResources,
		)
		if err != nil {
			return LocalRun{}, errors.Wrap(err, "generating alias to resource map")
		}
		runConfig.Resources = resources
		run.RunID = runID
		run.CreatedAt = start
		run.ParamValues = req.ParamValues
		run.Parameters = &parameters
		run.TaskName = req.Slug
		run.Status = api.RunActive
		// if the user is authenticated in CLI, use their ID
		run.CreatorID = state.cliConfig.ParseTokenForAnalytics().UserID
		state.runs.add(req.Slug, runID, run)

		outputs, err := state.executor.Execute(ctx, runConfig)
		if err != nil {
			run.Status = api.RunFailed
		} else if err == nil {
			run.Status = api.RunSucceeded
		}
		run.Outputs = outputs
	} else {
		logger.Error("task with slug %s is not registered locally", req.Slug)
	}
	now := time.Now()
	if run.Status == api.RunSucceeded {
		run.SucceededAt = &now
	} else {
		run.FailedAt = &now
	}
	state.runs.update(runID, run)
	return run, nil
}

// GetTaskMetadataHandler handles requests to the /v0/tasks/metadata endpoint. It generates a deterministic task ID for
// each task found locally, and its primary purpose is to ensure that the task discoverer does not error.
func GetTaskMetadataHandler(ctx context.Context, state *State, r *http.Request) (libapi.TaskMetadata, error) {
	slug := r.URL.Query().Get("slug")
	return libapi.TaskMetadata{
		ID:   fmt.Sprintf("tsk-%s", slug),
		Slug: slug,
	}, nil
}

// GetViewHandler handles requests to the /v0/views/get endpoint. It generates a deterministic view ID for each view
// found locally, and its primary purpose is to ensure that the view discoverer does not error.
func GetViewHandler(ctx context.Context, state *State, r *http.Request) (libapi.View, error) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		return libapi.View{}, errors.New("slug cannot be empty")
	}

	return libapi.View{
		ID:   fmt.Sprintf("vew-%s", slug),
		Slug: r.URL.Query().Get("slug"),
	}, nil
}

// GetRunHandler handles requests to the /v0/runs/get endpoint
func GetRunHandler(ctx context.Context, state *State, r *http.Request) (LocalRun, error) {
	runID := r.URL.Query().Get("id")
	run, ok := state.runs.get(runID)
	if !ok {
		return LocalRun{}, errors.Errorf("run with id %s not found", runID)
	}
	return run, nil
}

type ListRunsResponse struct {
	Runs []LocalRun `json:"runs"`
}

func ListRunsHandler(ctx context.Context, state *State, r *http.Request) (ListRunsResponse, error) {
	taskSlug := r.URL.Query().Get("taskSlug")
	runs := state.runs.getRunHistory(taskSlug)
	return ListRunsResponse{
		Runs: runs,
	}, nil
}

type GetOutputsResponse struct {
	// Outputs from this run.
	Output api.Outputs `json:"output"`
}

// GetOutputsHandler handles requests to the /v0/runs/getOutputs endpoint
func GetOutputsHandler(ctx context.Context, state *State, r *http.Request) (GetOutputsResponse, error) {
	runID := r.URL.Query().Get("id")
	run, ok := state.runs.get(runID)
	if !ok {
		return GetOutputsResponse{}, errors.Errorf("run with id %s not found", runID)
	}

	return GetOutputsResponse{
		Output: run.Outputs,
	}, nil
}

// GetTaskInfoHandler handles requests to the /v0/tasks?slug=<task_slug> endpoint.
func GetTaskInfoHandler(ctx context.Context, state *State, r *http.Request) (libapi.UpdateTaskRequest, error) {
	taskSlug := r.URL.Query().Get("slug")
	if taskSlug == "" {
		return libapi.UpdateTaskRequest{}, errors.New("Task slug was not supplied, request path must be of the form /v0/tasks?slug=<task_slug>")
	}
	taskConfig, ok := state.taskConfigs[taskSlug]
	if !ok {
		return libapi.UpdateTaskRequest{}, errors.Errorf("Task with slug %s not found", taskSlug)
	}
	req, err := taskConfig.Def.GetUpdateTaskRequest(ctx, state.cliConfig.Client)
	if err != nil {
		return libapi.UpdateTaskRequest{}, errors.Errorf("error getting task %s", taskSlug)
	}
	return req, nil
}

// ListResourcesHandler handles requests to the /v0/resources/list endpoint
func ListResourcesHandler(ctx context.Context, state *State, r *http.Request) (libapi.ListResourcesResponse, error) {
	resources := make([]libapi.Resource, 0, len(state.devConfig.Resources))
	for slug := range state.devConfig.Resources {
		// It doesn't matter what we include in the resource struct, as long as we include the slug - this handler
		// is only used so that requests to the local dev api server for this endpoint don't error, in particular:
		// https://github.com/airplanedev/lib/blob/d4c8ed7d1b30095c5cacac2b5c4da8f3ada6378f/pkg/deploy/taskdir/definitions/def_0_3.go#L1081-L1087
		resources = append(resources, libapi.Resource{
			Slug: slug,
		})
	}

	return libapi.ListResourcesResponse{
		Resources: resources,
	}, nil
}
