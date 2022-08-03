package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/airplanedev/cli/pkg/views"
	"github.com/airplanedev/cli/pkg/views/viewdir"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// AttachDevRoutes attaches the endpoints necessary to locally develop a task in the Airplane IDE.
func AttachDevRoutes(r *mux.Router, state *State) {
	const basePath = "/dev/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/ping", PingHandler()).Methods("GET", "OPTIONS")
	r.Handle("/list", ListAppMetadataHandler(state)).Methods("GET", "OPTIONS")
	r.Handle("/tasks/{task_slug}", GetTaskHandler(state)).Methods("GET", "OPTIONS")
	r.Handle("/startView/{view_slug}", StartViewHandler(state)).Methods("POST", "OPTIONS")
}

// PingHandler handles request to the /dev/ping endpoint.
func PingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}
}

type AppKind string

const (
	AppKindTask = "task"
	AppKindView = "view"
)

// AppMetadata represents metadata for a task or view.
type AppMetadata struct {
	Name string  `json:"name"`
	Slug string  `json:"slug"`
	Kind AppKind `json:"kind"`
}

type ListAppMetadataHandlerResponse struct {
	Tasks []AppMetadata `json:"tasks"`
	Views []AppMetadata `json:"views"`
}

// ListAppMetadataHandler handles requests to the /dev/list endpoint.
func ListAppMetadataHandler(state *State) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		tasks := make([]AppMetadata, 0, len(state.taskConfigs))
		for slug, taskConfig := range state.taskConfigs {
			tasks = append(tasks, AppMetadata{
				Name: taskConfig.Def.GetName(),
				Slug: slug,
				Kind: AppKindTask,
			})
		}

		views := make([]AppMetadata, 0, len(state.viewConfigs))
		for slug, viewConfig := range state.viewConfigs {
			views = append(views, AppMetadata{
				Name: viewConfig.Def.Name,
				Slug: slug,
				Kind: AppKindView,
			})
		}

		return json.NewEncoder(w).Encode(ListAppMetadataHandlerResponse{
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

type StartViewResponse struct {
	ViteServer string `json:"viteServer"`
}

// StartViewHandler handles requests to the /dev/tasks/<task_slug> endpoint.
func StartViewHandler(state *State) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		// TODO: Maintain mapping between view and vite process instead of starting a new vite process on each request.
		state.viteMutex.Lock()
		defer state.viteMutex.Unlock()
		if state.viteProcess != nil {
			if err := state.viteProcess.Kill(); err != nil {
				return errors.Wrap(err, "killing previous vite process")
			}
		}

		vars := mux.Vars(r)
		viewSlug, ok := vars["view_slug"]
		if !ok {
			return errors.Errorf("View slug was not supplied, request path must be of the form /dev/startView/<view_slug>")
		}

		viewConfig, ok := state.viewConfigs[viewSlug]
		if !ok {
			return errors.Errorf("View with slug %s not found", viewSlug)
		}

		rootDir, err := viewdir.FindRoot(viewConfig.Root)
		if err != nil {
			return err
		}

		vd, err := viewdir.NewViewDirectory(ctx, state.cliConfig, rootDir, viewConfig.Root, state.envSlug)
		if err != nil {
			return err
		}

		cmd, viteServer, err := views.Dev(vd, views.ViteOpts{
			Client:  state.cliConfig.Client,
			EnvSlug: state.envSlug,
			TTY:     false,
		})
		if err != nil {
			return errors.Wrap(err, "starting views dev")
		}

		state.viteProcess = cmd.Process

		u, err := url.Parse(viteServer)
		if err != nil {
			return errors.Wrapf(err, "parsing vite server url %s", viteServer)
		}

		// Wait for Vite to become ready
		for {
			conn, _ := net.DialTimeout("tcp", u.Host, 10*time.Second)
			if conn != nil {
				conn.Close()
				break
			}
			time.Sleep(300 * time.Millisecond)
		}

		return json.NewEncoder(w).Encode(StartViewResponse{ViteServer: viteServer})
	})
}
