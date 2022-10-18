package apidev

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/server/dev_errors"
	"github.com/airplanedev/cli/pkg/server/handlers"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/version"
	"github.com/airplanedev/cli/pkg/version/latest"
	"github.com/airplanedev/cli/pkg/views"
	"github.com/airplanedev/cli/pkg/views/viewdir"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// AttachDevRoutes attaches the endpoints necessary to locally develop a task in the Airplane IDE.
func AttachDevRoutes(r *mux.Router, s *state.State) {
	const basePath = "/dev/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/version", handlers.Handler(s, GetVersionHandler)).Methods("GET", "OPTIONS")

	r.Handle("/list", handlers.Handler(s, ListEntrypointsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/startView/{view_slug}", handlers.Handler(s, StartViewHandler)).Methods("POST", "OPTIONS")
	r.Handle("/logs/{run_id}", handlers.HandlerSSE(s, LogsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/tasks/errors", handlers.Handler(s, GetTaskErrorsHandler)).Methods("GET", "OPTIONS")
}

func GetVersionHandler(ctx context.Context, s *state.State, r *http.Request) (version.Metadata, error) {
	if s.VersionCache.Version != nil {
		return *s.VersionCache.Version, nil
	}

	isLatest := latest.CheckLatest(ctx)
	v := version.Metadata{
		Status:   "ok",
		Version:  version.Get(),
		IsLatest: isLatest,
	}
	s.VersionCache.Add(v)
	return v, nil
}

type AppKind string

const (
	AppKindTask = "task"
	AppKindView = "view"
)

// AppMetadata represents metadata for a task or view.
type AppMetadata struct {
	Name    string  `json:"name"`
	Slug    string  `json:"slug"`
	Kind    AppKind `json:"kind"`
	Runtime string  `json:"runtime"`
}

type ListEntrypointsHandlerResponse struct {
	Entrypoints map[string][]AppMetadata `json:"entrypoints"`
}

// ListEntrypointsHandler handles requests to the /dev/list endpoint. It generates a mapping from entrypoint relative to
// the dev server root to the list of tasks and views that use that entrypoint.
func ListEntrypointsHandler(ctx context.Context, state *state.State, r *http.Request) (ListEntrypointsHandlerResponse, error) {
	entrypoints := make(map[string][]AppMetadata)

	for slug, taskConfig := range state.TaskConfigs.Items() {
		absoluteEntrypoint := taskConfig.TaskEntrypoint
		if absoluteEntrypoint == "" {
			// for YAML-only tasks like REST that don't have entrypoints
			// display the yaml path instead
			absoluteEntrypoint = taskConfig.Def.GetDefnFilePath()
		}
		ep, err := filepath.Rel(state.Dir, absoluteEntrypoint)
		if err != nil {
			return ListEntrypointsHandlerResponse{}, errors.Wrap(err, "getting relative path to task")
		}
		if _, ok := entrypoints[ep]; !ok {
			entrypoints[ep] = make([]AppMetadata, 0, 1)
		}
		entrypoints[ep] = append(entrypoints[ep], AppMetadata{
			Name:    taskConfig.Def.GetName(),
			Slug:    slug,
			Kind:    AppKindTask,
			Runtime: string(taskConfig.Def.GetRuntime()),
		})
	}

	for slug, viewConfig := range state.ViewConfigs.Items() {
		absoluteEntrypoint := viewConfig.Def.Entrypoint

		ep, err := filepath.Rel(state.Dir, absoluteEntrypoint)
		if err != nil {
			return ListEntrypointsHandlerResponse{}, errors.Wrap(err, "getting relative path to view")
		}
		if _, ok := entrypoints[ep]; !ok {
			entrypoints[ep] = make([]AppMetadata, 0, 1)
		}
		entrypoints[ep] = append(entrypoints[ep], AppMetadata{
			Name: viewConfig.Def.Name,
			Slug: slug,
			Kind: AppKindView,
		})
	}

	return ListEntrypointsHandlerResponse{
		Entrypoints: entrypoints,
	}, nil
}

type CreateRunResponse struct {
	RunID string `json:"runID"`
}

type CreateRunRequest struct {
	TaskSlug string `json:"taskSlug"`
}

type StartViewResponse struct {
	ViteServer string `json:"viteServer"`
}

// StartViewHandler handles requests to the /dev/tasks/<task_slug> endpoint.
func StartViewHandler(ctx context.Context, s *state.State, r *http.Request) (StartViewResponse, error) {
	vars := mux.Vars(r)
	viewSlug, ok := vars["view_slug"]
	if !ok {
		return StartViewResponse{}, errors.Errorf("View slug was not supplied, request path must be of the form /dev/startView/<view_slug>")
	}

	viteContext, ok := s.ViteContexts.Get(viewSlug)
	if ok {
		contextObj, ok := viteContext.(state.ViteContext)
		if !ok {
			logger.Error("expected vite context from context cache")
			return StartViewResponse{}, errors.New("Could not obtain vite process")
		}
		return StartViewResponse{ViteServer: contextObj.ServerURL}, nil
	}

	viewConfig, ok := s.ViewConfigs.Get(viewSlug)
	if !ok {
		return StartViewResponse{}, errors.Errorf("View with slug %s not found", viewSlug)
	}

	rootDir, err := viewdir.FindRoot(viewConfig.Root)
	if err != nil {
		return StartViewResponse{}, err
	}

	vd, err := viewdir.NewViewDirectory(ctx, s.LocalClient, rootDir, viewConfig.Def.DefnFilePath, "")
	if err != nil {
		return StartViewResponse{}, err
	}

	cmd, viteServer, closer, err := views.Dev(ctx, &vd, views.ViteOpts{
		Client: s.LocalClient,
		TTY:    false,
	})
	if err != nil {
		return StartViewResponse{}, errors.Wrap(err, "starting views dev")
	}

	u, err := url.Parse(viteServer)
	if err != nil {
		return StartViewResponse{}, errors.Wrapf(err, "parsing vite server url %s", viteServer)
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

	s.ViteContexts.Add(viewSlug, state.ViteContext{
		Process:   cmd.Process,
		Closer:    closer,
		ServerURL: viteServer,
	})

	return StartViewResponse{ViteServer: viteServer}, nil
}

func LogsHandler(ctx context.Context, state *state.State, r *http.Request, flush func(log api.LogItem) error) error {
	vars := mux.Vars(r)
	runID, ok := vars["run_id"]
	if !ok {
		return errors.Errorf("Run id was not supplied, request path must be of the form /dev/logs/<run_id>")
	}

	run, ok := state.Runs.Get(runID)
	if !ok {
		return errors.Errorf("Run with id %s not found", runID)
	}

	watcher := run.LogBroker.NewWatcher()
	defer watcher.Close()
	for {
		select {
		// If the client has closed their request, then we unregister the current watcher.
		case <-ctx.Done():
			return nil
		case log, open := <-watcher.Logs():
			if !open {
				// All logs have been received.
				return nil
			}
			if err := flush(log); err != nil {
				return err
			}
		}
	}
}

type GetTaskErrorResponse struct {
	Errors   []dev_errors.AppError `json:"errors"`
	Warnings []dev_errors.AppError `json:"warnings"`
	Info     []dev_errors.AppError `json:"info"`
}

// GetTaskErrorsHandler returns any errors found for the task, grouped by level
func GetTaskErrorsHandler(ctx context.Context, state *state.State, r *http.Request) (GetTaskErrorResponse, error) {
	taskSlug := r.URL.Query().Get("slug")
	if taskSlug == "" {
		return GetTaskErrorResponse{}, errors.New("Task slug was not supplied, request path must be of the form /v0/tasks/warnings?slug=<task_slug>")
	}
	allErrors, ok := state.TaskErrors.Get(taskSlug)
	if !ok {
		return GetTaskErrorResponse{}, nil
	}
	warnings := []dev_errors.AppError{}
	errors := []dev_errors.AppError{}
	info := []dev_errors.AppError{}

	for _, e := range allErrors {
		if e.Level == dev_errors.LevelWarning {
			warnings = append(warnings, e)
		} else if e.Level == dev_errors.LevelError {
			errors = append(errors, e)
		} else if e.Level == dev_errors.LevelInfo {
			info = append(info, e)
		}
	}
	return GetTaskErrorResponse{Info: info, Errors: errors, Warnings: warnings}, nil
}
