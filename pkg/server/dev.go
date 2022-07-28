package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// AttachDevRoutes attaches the endpoints necessary to locally develop a task in the Airplane IDE.
func AttachDevRoutes(r *mux.Router, state *State) {
	const basePath = "/dev/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/ping", PingHandler()).Methods("GET", "OPTIONS")
	r.Handle("/list", ListDevMetadataHandler(state)).Methods("GET", "OPTIONS")
	r.Handle("/tasks/{task_slug}", GetTaskHandler(state)).Methods("GET", "OPTIONS")
}

// PingHandler handles request to the /dev/ping endpoint.
func PingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}
}

// DevMetadata represents metadata for a task or view.
type DevMetadata struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ListDevMetadataHandlerResponse struct {
	// Outputs from this run.
	Tasks []DevMetadata `json:"tasks"`
	Views []DevMetadata `json:"views"`
}

// ListDevMetadataHandler handles requests to the /dev/list endpoint.
func ListDevMetadataHandler(state *State) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		tasks := make([]DevMetadata, 0, len(state.taskConfigs))
		for slug, taskConfig := range state.taskConfigs {
			tasks = append(tasks, DevMetadata{
				Name: taskConfig.Def.GetName(),
				Slug: slug,
			})
		}

		views := make([]DevMetadata, 0, len(state.viewConfigs))
		for slug, viewConfig := range state.viewConfigs {
			views = append(views, DevMetadata{
				Name: viewConfig.Def.Name,
				Slug: slug,
			})
		}

		return json.NewEncoder(w).Encode(ListDevMetadataHandlerResponse{
			Tasks: tasks,
			Views: views,
		})
	})
}

// GetTaskHandler handles requests to the /dev/tasks/<task_slug> endpoint.
func GetTaskHandler(state *State) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		vars := mux.Vars(r)
		taskSlug, ok := vars["task_slug"]
		if !ok {
			return errors.Errorf("Task slug was not supplied, request path must be of the form /dev/tasks/<task_slug>")
		}

		taskConfig, ok := state.taskConfigs[taskSlug]
		if !ok {
			return errors.Errorf("Task with slug %s not found", taskSlug)
		}

		return json.NewEncoder(w).Encode(taskConfig.Def)
	})
}
