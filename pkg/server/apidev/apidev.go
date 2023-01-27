package apidev

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/server/dev_errors"
	"github.com/airplanedev/cli/pkg/server/handlers"
	"github.com/airplanedev/cli/pkg/server/network"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/version"
	"github.com/airplanedev/cli/pkg/version/latest"
	"github.com/airplanedev/cli/pkg/views"
	"github.com/airplanedev/cli/pkg/views/viewdir"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/build/ignore"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// AttachDevRoutes attaches the endpoints necessary to locally develop a task in the Airplane IDE.
func AttachDevRoutes(r *mux.Router, s *state.State) {
	const basePath = "/dev/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/info", handlers.Handler(s, GetInfoHandler)).Methods("GET", "OPTIONS")
	r.Handle("/version", handlers.Handler(s, GetVersionHandler)).Methods("GET", "OPTIONS")

	r.Handle("/envVars/get", handlers.Handler(s, GetEnvVarHandler)).Methods("GET", "OPTIONS")
	r.Handle("/envVars/upsert", handlers.HandlerWithBody(s, UpsertEnvVarHandler)).Methods("PUT", "OPTIONS")
	r.Handle("/envVars/delete", handlers.HandlerWithBody(s, DeleteEnvVarHandler)).Methods("DELETE", "OPTIONS")
	r.Handle("/envVars/list", handlers.Handler(s, ListEnvVarsHandler)).Methods("GET", "OPTIONS")

	// TODO: Remove this endpoint once the studio UI is updated to use /dev/files/list
	r.Handle("/list", handlers.Handler(s, ListEntrypointsHandler)).Methods("GET", "OPTIONS")

	r.Handle("/files/list", handlers.Handler(s, ListFilesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/files/get", handlers.Handler(s, GetFileHandler)).Methods("GET", "OPTIONS")
	r.Handle("/files/update", handlers.HandlerWithBody(s, UpdateFileHandler)).Methods("POST", "OPTIONS")
	r.Handle("/files/downloadBundle", handlers.HandlerZip(s, DownloadBundleHandler)).Methods("GET", "OPTIONS")

	r.Handle("/startView/{view_slug}", handlers.Handler(s, StartViewHandler)).Methods("POST", "OPTIONS")

	r.Handle("/logs/{run_id}", handlers.HandlerSSE(s, LogsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/tasks/errors", handlers.Handler(s, GetTaskErrorsHandler)).Methods("GET", "OPTIONS")

	r.PathPrefix("/views").HandlerFunc(ProxyViewHandler(s.PortProxy)).Methods("GET", "POST", "OPTIONS")
	r.Handle("/dependencies/reinstall", handlers.Handler(s, ReinstallDependenciesHandler)).Methods("POST", "OPTIONS")
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
	Workspace            string      `json:"workspace"`
	DefaultEnv           libapi.Env  `json:"defaultEnv"`
	FallbackEnv          *libapi.Env `json:"fallbackEnv"`
	Host                 string      `json:"host"`
	IsSandbox            bool        `json:"isSandbox"`
	IsRebuilding         bool        `json:"isRebuilding"`
	OutdatedDependencies bool        `json:"outdatedDependencies"`
	HasDependencyError   bool        `json:"hasDependencyError"`
}

func GetInfoHandler(ctx context.Context, s *state.State, r *http.Request) (StudioInfo, error) {
	var fallbackEnv *libapi.Env
	if s.UseFallbackEnv {
		fallbackEnv = &s.RemoteEnv
	}

	// TODO: Fix default env and host for remote studio.
	info := StudioInfo{
		Workspace:   s.Dir,
		DefaultEnv:  env.NewLocalEnv(),
		FallbackEnv: fallbackEnv,
		Host:        strings.Replace(s.LocalClient.Host, "127.0.0.1", "localhost", 1),
	}

	if s.SandboxState != nil {
		info.IsSandbox = true
		info.IsRebuilding = s.SandboxState.IsRebuilding
		info.OutdatedDependencies = s.SandboxState.OutdatedDependencies
		info.HasDependencyError = s.SandboxState.HasDependencyError
	}

	return info, nil
}

type EntityKind string

const (
	EntityKindTask = "task"
	EntityKindView = "view"
)

// EntityMetadata represents metadata for a task or view.
type EntityMetadata struct {
	Name    string            `json:"name"`
	Slug    string            `json:"slug"`
	Kind    EntityKind        `json:"kind"`
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
			Kind:    EntityKindTask,
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
			Kind: EntityKindView,
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
				Kind:    EntityKindTask,
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
	IsDir    bool             `json:"isDir"`
	Children []*FileNode      `json:"children"`
	Entities []EntityMetadata `json:"entities"`
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
			Kind:    EntityKindTask,
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
			Kind: EntityKindView,
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
			var isDir bool
			if info.IsDir() {
				base := filepath.Base(path)
				if _, ok := ignoredDirs[base]; ok {
					return filepath.SkipDir
				}
				isDir = true
			}

			nodes[path] = &FileNode{
				Path:     path,
				IsDir:    isDir,
				Children: make([]*FileNode, 0),
				Entities: filepathToEntities[path],
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

	if strings.Contains(path, "..") {
		return GetFileResponse{}, errors.New("path may not contain directory traversal elements (`..`)")
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return GetFileResponse{}, errors.Wrap(err, "reading file")
	}

	return GetFileResponse{Content: string(contents)}, nil
}

type UpdateFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// UpdateFileHandler updates the file at the requested location. It takes in the new file contents in the request body,
// and updates the specified file only if the file path is within the development root.
func UpdateFileHandler(ctx context.Context, s *state.State, r *http.Request, req UpdateFileRequest) (struct{}, error) {
	// Ensure the path is within the dev server root.
	if !strings.HasPrefix(req.Path, s.Dir) {
		return struct{}{}, errors.New("path is outside dev root")
	}

	if strings.Contains(req.Path, "..") {
		return struct{}{}, errors.New("path may not contain directory traversal elements (`..`)")
	}

	if err := os.WriteFile(req.Path, []byte(req.Content), 0644); err != nil {
		return struct{}{}, errors.Wrap(err, "writing file")
	}

	// TODO: check embedded requirements
	if s.SandboxState != nil {
		base := filepath.Base(req.Path)
		if base == "requirements.txt" || base == "package.json" || base == "yarn.lock" || base == "package-lock.json" {
			s.SandboxState.MarkDependenciesOutdated()
		}
	}

	return struct{}{}, nil
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

	viewConfig, ok := s.ViewConfigs.Get(viewSlug)
	if !ok {
		return StartViewResponse{}, errors.Errorf("View with slug %s not found", viewSlug)
	}

	vd, err := viewdir.NewViewDirectoryFromViewConfig(viewConfig)
	if err != nil {
		return StartViewResponse{}, err
	}

	// Vite has some caching logic that attempts to avoid bundling dependencies that haven't changed. However, we rely
	// on the root package.json file for non-vite dependencies, and Vite doesn't know about these and hence may not
	// re-bundle when they (e.g. @airplane/views) change. To work around this, we add a hash of the package.json file
	// to the .airplane-view directory, and check that the hash has changed before running Vite. If it has, we pass
	// the --force flag to Vite to force a re-bundle.
	usesYarn, depHashesEqual, err := views.CheckLockfile(&vd, s.Logger)
	if err != nil {
		return StartViewResponse{}, err
	}

	if depHashesEqual {
		// Check if the vite process for this view is already running.
		viteContext, ok := s.ViteContexts.Get(viewSlug)
		if ok {
			logger.Debug("Vite process already running for view %s", viewSlug)
			return StartViewResponse{ViteServer: viteContext.ServerURL}, nil
		}
	} else {
		// Invalidate all vite processes that use the same root
		for slug, cfg := range s.ViewConfigs.Items() {
			if slug == viewSlug {
				continue
			}

			if cfg.Root == viewConfig.Root {
				s.ViteContexts.Remove(slug)
			}
		}
	}

	// client represents the client that Vite will use to communicate with the dev server.
	client := s.LocalClient
	var port int
	var serverURL string
	if s.ServerHost != "" {
		// This is potentially subject to a race condition, but we need to allocate the port before starting Vite in
		// order to construct Vite config options.
		port, err = network.FindOpenPort()
		if err != nil {
			return StartViewResponse{}, err
		}
		serverURL = fmt.Sprintf("%s/dev/views/%d/", s.ServerHost, port)
		// If a server host is specified, we send (Airplane) API requests to that host.
		client = &api.Client{
			ClientOpts: api.ClientOpts{
				Host: s.ServerHost,
			},
		}
	}

	cmd, viteServer, closer, err := views.Dev(ctx, &vd, views.ViteOpts{
		Client:               client,
		TTY:                  false,
		RebundleDependencies: !depHashesEqual,
		UsesYarn:             usesYarn,
		Port:                 port,
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

	// If no address is set, we don't fix a port for the vite server, and so we just use the URL that the vite command
	// returns.
	if s.ServerHost == "" {
		serverURL = viteServer
	}

	s.ViteContexts.Add(viewSlug, state.ViteContext{
		Process:   cmd.Process,
		Closer:    closer,
		ServerURL: serverURL,
	})

	return StartViewResponse{ViteServer: serverURL}, nil
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

type ReinstallDependenciesResponse struct {
	Status ReinstallDependenciesStatus `json:"status"`
}

type ReinstallDependenciesStatus string

const (
	ReinstallDependenciesStatusDone       ReinstallDependenciesStatus = "done"
	ReinstallDependenciesStatusInProgress ReinstallDependenciesStatus = "in progress"
)

func ReinstallDependenciesHandler(ctx context.Context, s *state.State, r *http.Request) (ReinstallDependenciesResponse, error) {
	inProgress := s.SandboxState.RebuildWithTimeout(ctx, s.BundleDiscoverer, s.Dir)
	resp := ReinstallDependenciesResponse{}
	if inProgress {
		resp.Status = ReinstallDependenciesStatusInProgress
	} else {
		resp.Status = ReinstallDependenciesStatusDone
	}
	return resp, nil
}

func DownloadBundleHandler(ctx context.Context, state *state.State, r *http.Request) ([]byte, string, error) {
	buf := new(bytes.Buffer)
	include, err := ignore.Func(state.Dir)
	if err != nil {
		return nil, "", errors.Wrap(err, "creating include function")
	}

	if err := utils.Zip(buf, state.Dir, include); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), filepath.Base(state.Dir), nil
}

// ProxyViewHandler proxies requests to the Vite server for a view.
func ProxyViewHandler(portProxy *httputil.ReverseProxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		portProxy.ServeHTTP(w, r)
	}
}
