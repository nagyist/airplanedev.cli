package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/resource"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/r3labs/sse/v2"
)

type LocalRun struct {
	status  api.RunStatus
	outputs api.Outputs
}

type State struct {
	cli         *cli.Config
	envSlug     string
	executor    dev.Executor
	port        int
	runs        map[string]LocalRun
	taskConfigs map[string]discover.TaskConfig
	devConfig   conf.DevConfig
	sseServer   *sse.Server
}

var upgrader = websocket.Upgrader{}

// AttachAPIRoutes attaches the endpoints necessary to locally develop a task. It is a minimal subset of the actual
// Airplane API.
func AttachAPIRoutes(r *mux.Router, ctx context.Context, state *State) {
	const basePath = "/v0/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/tasks/execute", ExecuteTaskHandler(ctx, state)).Methods("POST", "OPTIONS")
	r.Handle("/tasks/getMetadata", GetTaskMetadataHandler(ctx, state)).Methods("GET", "OPTIONS")
	r.Handle("/runs/getOutputs", GetOutputsHandler(ctx, state)).Methods("GET", "OPTIONS")
	r.Handle("/runs/get", GetRunHandler(ctx, state)).Methods("GET", "OPTIONS")
	r.Handle("/resources/list", ListResourcesHandler(ctx, state)).Methods("GET", "OPTIONS")
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
		outputs := api.Outputs{}
		if config, ok := state.taskConfigs[req.Slug]; ok {
			kind, kindOptions, err := config.Def.GetKindAndOptions()
			if err != nil {
				logger.Error("failed to get kind and options from task config")
				return
			}

			resources, err := resource.GenerateAliasToResourceMap(
				config.Def.GetResourceAttachments(),
				state.devConfig.DecodedResources,
			)
			if err != nil {
				logger.Error("generating alias to resource map")
				return
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
				logger.Error("failed to run task locally")
				return
			}

			runStatus = api.RunSucceeded
		} else {
			logger.Error("task with slug %s is not registered locally", req.Slug)
		}

		state.runs[runID] = LocalRun{
			status:  runStatus,
			outputs: outputs,
		}

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

		state.sseServer.CreateStream("messages")

		state.sseServer.Publish("messages", &sse.Event{
			Data: []byte("ping"),
		})

		run, ok := state.runs[runID]
		if !ok {
			logger.Error("run with id %s not found", runID)
			return
		}

		if err := json.NewEncoder(w).Encode(GetRunResponse{
			ID:     runID,
			Status: run.status,
		}); err != nil {
			logger.Error("failed to encode response for /v0/runs/get")
		}
	}
}

type GetOutputsResponse struct {
	// Outputs from this run.
	Output api.Outputs `json:"output"`
}

// GetOutputsHandler handles requests to the /v0/runs/getOutputs endpoint
func GetOutputsHandler(ctx context.Context, state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		defer c.Close()
		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", message)
			err = c.WriteMessage(mt, message)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}

		runID := r.URL.Query().Get("id")
		run, ok := state.runs[runID]
		if !ok {
			logger.Error("run with id %s not found", runID)
			return
		}

		if err := json.NewEncoder(w).Encode(GetOutputsResponse{
			Output: run.outputs,
		}); err != nil {
			logger.Error("failed to encode response for /v0/runs/getOutputs")
		}
	}
}

// ListResourcesHandler handles requests to the /v0/resources/list endpoint
func ListResourcesHandler(ctx context.Context, state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resources := make([]libapi.Resource, 0, len(state.devConfig.Resources))
		for slug := range state.devConfig.Resources {
			// It doesn't matter what we include in the resource struct, as long as we include the slug - this handler
			// is only used so that requests to the local dev api server for this endpoint don't error, in particular:
			// https://github.com/airplanedev/lib/blob/d4c8ed7d1b30095c5cacac2b5c4da8f3ada6378f/pkg/deploy/taskdir/definitions/def_0_3.go#L1081-L1087
			resources = append(resources, libapi.Resource{
				Slug: slug,
			})
		}

		if err := json.NewEncoder(w).Encode(libapi.ListResourcesResponse{
			Resources: resources,
		}); err != nil {
			logger.Error("failed to encode response for /v0/resources/list")
		}
	}
}
