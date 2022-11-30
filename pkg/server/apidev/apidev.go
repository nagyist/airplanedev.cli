package apidev

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/server/dev_errors"
	"github.com/airplanedev/cli/pkg/server/handlers"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/version"
	"github.com/airplanedev/cli/pkg/version/latest"
	"github.com/airplanedev/cli/pkg/views"
	"github.com/airplanedev/cli/pkg/views/viewdir"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// AttachDevRoutes attaches the endpoints necessary to locally develop a task in the Airplane IDE.
func AttachDevRoutes(r *mux.Router, s *state.State) {
	const basePath = "/dev/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/info", handlers.Handler(s, GetInfoHandler)).Methods("GET", "OPTIONS")
	r.Handle("/version", handlers.Handler(s, GetVersionHandler)).Methods("GET", "OPTIONS")

	// TODO: Remove this endpoint once the studio UI is updated to use /dev/files/list
	r.Handle("/list", handlers.Handler(s, ListEntrypointsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/files/list", handlers.Handler(s, ListFilesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/files/get", handlers.Handler(s, GetFileHandler)).Methods("GET", "OPTIONS")

	r.Handle("/startView/{view_slug}", handlers.Handler(s, StartViewHandler)).Methods("POST", "OPTIONS")
	r.Handle("/logs/{run_id}", handlers.HandlerSSE(s, LogsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/tasks/errors", handlers.Handler(s, GetTaskErrorsHandler)).Methods("GET", "OPTIONS")
}

func GetVersionHandler(ctx context.Context, s *state.State, r *http.Request) (version.Metadata, error) {
	if s.VersionCache.Version != nil {
		return *s.VersionCache.Version, nil
	}

	isLatest := latest.CheckLatest(ctx, nil)
	v := version.Metadata{
		Status:   "ok",
		Version:  version.Get(),
		IsLatest: isLatest,
	}
	s.VersionCache.Add(v)
	return v, nil
}

type StudioInfo struct {
	Workspace   string      `json:"workspace"`
	DefaultEnv  libapi.Env  `json:"defaultEnv"`
	FallbackEnv *libapi.Env `json:"fallbackEnv"`
	Host        string      `json:"host"`
}

func GetInfoHandler(ctx context.Context, s *state.State, r *http.Request) (StudioInfo, error) {
	var fallbackEnv *libapi.Env
	if s.UseFallbackEnv {
		fallbackEnv = &s.RemoteEnv
	}

	return StudioInfo{
		Workspace:   s.Dir,
		DefaultEnv:  env.NewLocalEnv(),
		FallbackEnv: fallbackEnv,
		Host:        strings.Replace(s.LocalClient.Host, "127.0.0.1", "localhost", 1),
	}, nil
}

type AppKind string

const (
	AppKindTask = "task"
	AppKindView = "view"
)

// EntityMetadata represents metadata for a task or view.
type EntityMetadata struct {
	Name    string            `json:"name"`
	Slug    string            `json:"slug"`
	Kind    AppKind           `json:"kind"`
	Runtime build.TaskRuntime `json:"runtime"`
}

type ListEntrypointsHandlerResponse struct {
	Entrypoints       map[string][]EntityMetadata `json:"entrypoints"`
	RemoteEntrypoints []EntityMetadata            `json:"remoteEntrypoints"`
}

// ListEntrypointsHandler handles requests to the /dev/list endpoint. It generates a mapping from entrypoint relative to
// the dev server root to the list of tasks and views that use that entrypoint.
func ListEntrypointsHandler(ctx context.Context, state *state.State, r *http.Request) (ListEntrypointsHandlerResponse, error) {
	entrypoints := make(map[string][]EntityMetadata)
	remoteEntrypoints := make([]EntityMetadata, 0)

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
			entrypoints[ep] = make([]EntityMetadata, 0, 1)
		}
		entrypoints[ep] = append(entrypoints[ep], EntityMetadata{
			Name:    taskConfig.Def.GetName(),
			Slug:    slug,
			Kind:    AppKindTask,
			Runtime: taskConfig.Def.GetRuntime(),
		})
	}

	for slug, viewConfig := range state.ViewConfigs.Items() {
		absoluteEntrypoint := viewConfig.Def.Entrypoint

		ep, err := filepath.Rel(state.Dir, absoluteEntrypoint)
		if err != nil {
			return ListEntrypointsHandlerResponse{}, errors.Wrap(err, "getting relative path to view")
		}
		if _, ok := entrypoints[ep]; !ok {
			entrypoints[ep] = make([]EntityMetadata, 0, 1)
		}
		entrypoints[ep] = append(entrypoints[ep], EntityMetadata{
			Name: viewConfig.Def.Name,
			Slug: slug,
			Kind: AppKindView,
		})
	}

	// List remote entrypoints if fallback env is specified
	if state.UseFallbackEnv {
		res, err := state.RemoteClient.ListTasks(ctx, state.RemoteEnv.Slug)
		if err != nil {
			return ListEntrypointsHandlerResponse{}, errors.Wrap(err, "getting remote tasks")
		}

		for _, task := range res.Tasks {
			remoteEntrypoints = append(remoteEntrypoints, EntityMetadata{
				Name:    task.Name,
				Slug:    task.Slug,
				Kind:    AppKindTask,
				Runtime: task.Runtime,
			})
		}
	}

	return ListEntrypointsHandlerResponse{
		Entrypoints:       entrypoints,
		RemoteEntrypoints: remoteEntrypoints,
	}, nil
}

type FileNode struct {
	Path     string           `json:"path"`
	Entities []EntityMetadata `json:"entities"`
	Children []*FileNode      `json:"children"`
}

type ListFilesResponse struct {
	Root *FileNode `json:"root"`
}

var ignoredDirs = map[string]struct{}{
	".airplane":      {},
	".airplane-view": {},
	"node_modules":   {},
	"__pycache__":    {},
	"venv":           {},
}

// ListFilesHandler handles requests to the /dev/files/list endpoint. It generates a tree of all files under the dev
// server root, with entities that are declared in each file.
func ListFilesHandler(ctx context.Context, state *state.State, r *http.Request) (ListFilesResponse, error) {
	// Track entities per file, which we'll use to show entities in the UI.
	filepathToEntities := make(map[string][]EntityMetadata, 0)
	for slug, taskConfig := range state.TaskConfigs.Items() {
		defnFilePath := taskConfig.Def.GetDefnFilePath()
		if _, ok := filepathToEntities[defnFilePath]; !ok {
			filepathToEntities[defnFilePath] = make([]EntityMetadata, 0, 1)
		}
		filepathToEntities[defnFilePath] = append(filepathToEntities[defnFilePath], EntityMetadata{
			Name:    taskConfig.Def.GetName(),
			Slug:    slug,
			Kind:    AppKindTask,
			Runtime: taskConfig.Def.GetRuntime(),
		})
	}

	for slug, viewConfig := range state.ViewConfigs.Items() {
		defnFilePath := viewConfig.Def.DefnFilePath
		if _, ok := filepathToEntities[defnFilePath]; !ok {
			filepathToEntities[defnFilePath] = make([]EntityMetadata, 0, 1)
		}
		filepathToEntities[defnFilePath] = append(filepathToEntities[defnFilePath], EntityMetadata{
			Name: viewConfig.Def.Name,
			Slug: slug,
			Kind: AppKindView,
		})
	}

	// Track all file tree nodes. We'll use this to build the file tree. Inspired by https://github.com/marcinwyszynski/directory_tree
	nodes := make(map[string]*FileNode)
	if err := filepath.Walk(state.Dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Ignore non-user-facing directories.
			if info.IsDir() {
				base := filepath.Base(path)
				if _, ok := ignoredDirs[base]; ok {
					return filepath.SkipDir
				}
			}

			nodes[path] = &FileNode{
				Path:     path,
				Entities: filepathToEntities[path],
				Children: make([]*FileNode, 0),
			}
			return nil
		},
	); err != nil {
		return ListFilesResponse{}, err
	}

	// Construct directory tree.
	var root *FileNode
	for path, node := range nodes {
		parentDir := filepath.Dir(path)
		parent, ok := nodes[parentDir]
		if ok {
			parent.Children = append(parent.Children, node)
		} else {
			root = node
		}
	}

	return ListFilesResponse{Root: root}, nil
}

type GetFileResponse struct {
	Content string `json:"content"`
}

// GetFileHandler returns the contents of the file at the requested location. Path is the absolute path to the file,
// irrespective of the dev server root.
func GetFileHandler(ctx context.Context, state *state.State, r *http.Request) (GetFileResponse, error) {
	path := r.URL.Query().Get("path")
	if path == "" {
		return GetFileResponse{}, errors.New("path is required")
	}

	// Ensure the path is within the dev server root.
	if !strings.HasPrefix(path, state.Dir) {
		return GetFileResponse{}, errors.New("path is outside dev root")
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return GetFileResponse{}, errors.Wrap(err, "reading file")
	}

	return GetFileResponse{Content: string(contents)}, nil
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
		return StartViewResponse{ViteServer: viteContext.ServerURL}, nil
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
	metadata, ok := state.AppCondition.Get(taskSlug)
	if !ok {
		return GetTaskErrorResponse{}, nil
	}
	warnings := []dev_errors.AppError{}
	errors := []dev_errors.AppError{}
	info := []dev_errors.AppError{}

	for _, e := range metadata.Errors {
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
