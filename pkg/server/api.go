package server

import (
	"encoding/json"
	"net/http"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/ojson"
	"github.com/gorilla/mux"
)

// AttachAPIRoutes attaches the endpoints necessary to locally develop a task. It is a minimal subset of the actual
// Airplane API.
func AttachAPIRoutes(r *mux.Router) {
	const basePath = "/v0/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/tasks/execute", ExecuteTaskHandler()).Methods("POST", "OPTIONS")
	r.Handle("/runs/getOutputs", GetOutputsHandler()).Methods("GET", "OPTIONS")
	r.Handle("/runs/get", GetRunHandler()).Methods("GET", "OPTIONS")
}

type ExecuteTaskRequest struct {
	ID          *string                `json:"id"`
	Slug        *string                `json:"slug"`
	ParamValues map[string]interface{} `json:"paramValues"`
	Resources   map[string]string      `json:"resources"`
}

type ExecuteTaskResponse struct {
	RunID string `json:"runID"`
}

// ExecuteTaskHandler handles requests to the /v0/tasks/execute endpoint
func ExecuteTaskHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement logic to execute task in a child process
		if err := json.NewEncoder(w).Encode(ExecuteTaskResponse{
			RunID: "run" + utils.RandomString(10, utils.CharsetLowercaseNumeric),
		}); err != nil {
			logger.Error("failed to encode response for /v0/tasks/execute")
		}
	}
}

type GetRunResponse struct {
	ID string `json:"id"`
}

// GetRunHandler handles requests to the /v0/runs/get endpoint
func GetRunHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID := r.URL.Query().Get("id")
		if err := json.NewEncoder(w).Encode(GetRunResponse{
			ID: runID,
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
func GetOutputsHandler() http.HandlerFunc {
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
