package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/resource"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

type LocalRun struct {
	status  api.RunStatus
	outputs api.Outputs
}

// AttachAPIRoutes attaches a minimal subset of the actual Airplane API endpoints that are necessary to locally develop
// a task. For example, a workflow task might call airplane.execute, which would normally make a request to the
// /v0/tasks/execute endpoint in production, but instead we have our own implementation below.
func AttachAPIRoutes(r *mux.Router, state *State) {
	const basePath = "/v0/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/tasks/execute", ExecuteTaskHandler(state)).Methods("POST", "OPTIONS")
	r.Handle("/tasks/getMetadata", GetTaskMetadataHandler(state)).Methods("GET", "OPTIONS")
	r.Handle("/runs/getOutputs", GetOutputsHandler(state)).Methods("GET", "OPTIONS")
	r.Handle("/runs/get", GetRunHandler(state)).Methods("GET", "OPTIONS")
	r.Handle("/resources/list", ListResourcesHandler(state)).Methods("GET", "OPTIONS")
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
func ExecuteTaskHandler(state *State) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		var req ExecuteTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("failed to decode request body %+v", r.Body)
		}

		runID := "run" + utils.RandomString(10, utils.CharsetLowercaseNumeric)
		runStatus := api.RunFailed
		outputs := api.Outputs{}
		if config, ok := state.taskConfigs[req.Slug]; ok {
			kind, kindOptions, err := config.Def.GetKindAndOptions()
			if err != nil {
				return errors.Wrap(err, "failed to get kind and options from task config")
			}

			resources, err := resource.GenerateAliasToResourceMap(
				config.Def.GetResourceAttachments(),
				state.devConfig.DecodedResources,
			)
			if err != nil {
				return errors.Wrap(err, "generating alias to resource map")
			}

			outputs, err = state.executor.Execute(ctx, dev.LocalRunConfig{
				Name:        config.Def.GetName(),
				Kind:        kind,
				KindOptions: kindOptions,
				ParamValues: req.ParamValues,
				Port:        state.port,
				Root:        state.cli,
				File:        config.Def.GetDefnFilePath(),
				Slug:        req.Slug,
				EnvSlug:     state.envSlug,
				Resources:   resources,
			})
			if err != nil {
				return errors.Wrap(err, "failed to run task locally")
			}

			runStatus = api.RunSucceeded
		} else {
			logger.Error("task with slug %s is not registered locally", req.Slug)
		}

		state.runs[runID] = LocalRun{
			status:  runStatus,
			outputs: outputs,
		}

		return json.NewEncoder(w).Encode(ExecuteTaskResponse{
			RunID: runID,
		})
	})
}

// GetTaskMetadataHandler handles requests to the /v0/tasks/metadata endpoint. It generates a random task ID for each
// task found locally, and its primary purpose is to ensure that the task discoverer does not error.
func GetTaskMetadataHandler(state *State) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		slug := r.URL.Query().Get("slug")
		return json.NewEncoder(w).Encode(libapi.TaskMetadata{
			ID:   "tsk" + utils.RandomString(10, utils.CharsetLowercaseNumeric),
			Slug: slug,
		})
	})
}

type GetRunResponse struct {
	ID     string        `json:"id"`
	Status api.RunStatus `json:"status"`
}

// GetRunHandler handles requests to the /v0/runs/get endpoint
func GetRunHandler(state *State) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		runID := r.URL.Query().Get("id")

		run, ok := state.runs[runID]
		if !ok {
			return errors.Errorf("run with id %s not found", runID)
		}

		return json.NewEncoder(w).Encode(GetRunResponse{
			ID:     runID,
			Status: run.status,
		})
	})
}

type GetOutputsResponse struct {
	// Outputs from this run.
	Output api.Outputs `json:"output"`
}

// GetOutputsHandler handles requests to the /v0/runs/getOutputs endpoint
func GetOutputsHandler(state *State) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		runID := r.URL.Query().Get("id")
		run, ok := state.runs[runID]
		if !ok {
			return errors.Errorf("run with id %s not found", runID)
		}

		return json.NewEncoder(w).Encode(GetOutputsResponse{
			Output: run.outputs,
		})
	})
}

// ListResourcesHandler handles requests to the /v0/resources/list endpoint
func ListResourcesHandler(state *State) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		resources := make([]libapi.Resource, 0, len(state.devConfig.Resources))
		for slug := range state.devConfig.Resources {
			// It doesn't matter what we include in the resource struct, as long as we include the slug - this handler
			// is only used so that requests to the local dev api server for this endpoint don't error, in particular:
			// https://github.com/airplanedev/lib/blob/d4c8ed7d1b30095c5cacac2b5c4da8f3ada6378f/pkg/deploy/taskdir/definitions/def_0_3.go#L1081-L1087
			resources = append(resources, libapi.Resource{
				Slug: slug,
			})
		}

		return json.NewEncoder(w).Encode(libapi.ListResourcesResponse{
			Resources: resources,
		})
	})
}
