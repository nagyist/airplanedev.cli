package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/ojson"
	"github.com/gorilla/mux"
)

type State struct {
	cli         *cli.Config
	envSlug     string
	executor    dev.Executor
	port        int
	runs        map[string]api.RunStatus
	taskConfigs map[string]discover.TaskConfig
}

// AttachAPIRoutes attaches the endpoints necessary to locally develop a task. It is a minimal subset of the actual
// Airplane API.
func AttachAPIRoutes(r *mux.Router, ctx context.Context, state *State) {
	const basePath = "/v0/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/tasks/execute", ExecuteTaskHandler(ctx, state)).Methods("POST", "OPTIONS")
	r.Handle("/tasks/getMetadata", GetTaskMetadataHandler(ctx, state)).Methods("GET", "OPTIONS")
	r.Handle("/runs/getOutputs", GetOutputsHandler(ctx, state)).Methods("GET", "OPTIONS")
	r.Handle("/runs/get", GetRunHandler(ctx, state)).Methods("GET", "OPTIONS")
}

type ExecuteTaskRequest struct {
	Slug        string            `json:"slug"`
	ParamValues api.Values        `json:"paramValues"`
	Resources   map[string]string `json:"resources"`
}

type ExecuteTaskResponse struct {
	RunID string `json:"runID"`
}

// ExecuteTaskHandler handles requests to the /v0/tasks/execute endpoint
func ExecuteTaskHandler(ctx context.Context, state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ExecuteTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("failed to decode request body %+v", r.Body)
		}

		runID := "run" + utils.RandomString(10, utils.CharsetLowercaseNumeric)
		runStatus := api.RunFailed
		if config, ok := state.taskConfigs[req.Slug]; ok {
			kind, kindOptions, err := config.Def.GetKindAndOptions()
			if err != nil {
				logger.Error("failed to get kind and options from task config")
				return
			}

			if err := state.executor.Execute(ctx, dev.LocalRunConfig{
				Name:        config.Def.GetName(),
				Kind:        kind,
				KindOptions: kindOptions,
				ParamValues: req.ParamValues,
				Port:        state.port,
				Root:        state.cli,
				File:        config.Def.GetDefnFilePath(),
				Slug:        req.Slug,
				EnvSlug:     state.envSlug,
			}); err != nil {
				logger.Error("failed to run task locally")
				return
			}

			runStatus = api.RunSucceeded
		} else {
			logger.Error("task with slug %s is not registered locally", req.Slug)
		}

		state.runs[runID] = runStatus

		if err := json.NewEncoder(w).Encode(ExecuteTaskResponse{
			RunID: runID,
		}); err != nil {
			logger.Error("failed to encode response for /v0/tasks/execute")
		}
	}
}

// GetTaskMetadataHandler handles requests to the /v0/tasks/metadata endpoint. It generates a random task ID for each
// task found locally, and its primary purpose is to ensure that the task discoverer does not error.
func GetTaskMetadataHandler(ctx context.Context, state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("slug")
		if err := json.NewEncoder(w).Encode(libapi.TaskMetadata{
			ID:   "tsk" + utils.RandomString(10, utils.CharsetLowercaseNumeric),
			Slug: slug,
		}); err != nil {
			logger.Error("failed to encode response for /v0/tasks/getMetadata")
		}
	}
}

type GetRunResponse struct {
	ID     string        `json:"id"`
	Status api.RunStatus `json:"status"`
}

// GetRunHandler handles requests to the /v0/runs/get endpoint
func GetRunHandler(ctx context.Context, state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID := r.URL.Query().Get("id")

		status, ok := state.runs[runID]
		if !ok {
			logger.Error("status for run id %s not found", runID)
			return
		}

		if err := json.NewEncoder(w).Encode(GetRunResponse{
			ID:     runID,
			Status: status,
		}); err != nil {
			logger.Error("failed to encode response for /v0/runs/get")
		}
	}
}

type GetOutputsResponse struct {
	// Outputs from this run.
	Output ojson.Value `json:"output"`
}

// GetOutputsHandler handles requests to the /v0/runs/getOutputs endpoint
func GetOutputsHandler(ctx context.Context, state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(GetOutputsResponse{
			Output: ojson.Value{
				V: nil,
			},
		}); err != nil {
			logger.Error("failed to encode response for /v0/runs/getOutputs")
		}
	}
}
