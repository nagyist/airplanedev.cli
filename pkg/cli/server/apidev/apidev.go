package apidev

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	libviews "github.com/airplanedev/cli/pkg/build/views"
	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/cli/apiclient/http"
	"github.com/airplanedev/cli/pkg/cli/dev/env"
	"github.com/airplanedev/cli/pkg/cli/server/autopilot"
	"github.com/airplanedev/cli/pkg/cli/server/dev_errors"
	"github.com/airplanedev/cli/pkg/cli/server/handlers"
	"github.com/airplanedev/cli/pkg/cli/server/state"
	"github.com/airplanedev/cli/pkg/cli/server/status"
	serverutils "github.com/airplanedev/cli/pkg/cli/server/utils"
	"github.com/airplanedev/cli/pkg/cli/views"
	"github.com/airplanedev/cli/pkg/cli/views/viewdir"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

// AttachDevRoutes attaches the endpoints necessary to locally develop a task in the Airplane IDE.
func AttachDevRoutes(r *mux.Router, s *state.State) {
	const basePath = "/dev/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/info", handlers.New(s, GetInfoHandler)).Methods("GET", "OPTIONS")
	r.Handle("/version", handlers.New(s, GetVersionHandler)).Methods("GET", "OPTIONS")

	r.Handle("/envVars/get", handlers.New(s, GetEnvVarHandler)).Methods("GET", "OPTIONS")
	r.Handle("/envVars/upsert", handlers.WithBody(s, UpsertEnvVarHandler)).Methods("PUT", "OPTIONS")
	r.Handle("/envVars/delete", handlers.WithBody(s, DeleteEnvVarHandler)).Methods("DELETE", "OPTIONS")
	r.Handle("/envVars/list", handlers.New(s, ListEnvVarsHandler)).Methods("GET", "OPTIONS")

	// TODO: Remove this endpoint once the studio UI is updated to use /dev/files/list
	r.Handle("/list", handlers.New(s, ListEntrypointsHandler)).Methods("GET", "OPTIONS")

	r.Handle("/files/list", handlers.New(s, ListFilesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/files/get", handlers.New(s, GetFileHandler)).Methods("GET", "OPTIONS")
	r.Handle("/files/update", handlers.WithBody(s, UpdateFileHandler)).Methods("POST", "OPTIONS")
	r.Handle("/files/downloadBundle", handlers.Zip(s, DownloadBundleHandler)).Methods("GET", "OPTIONS")
	r.Handle("/files/patch", handlers.WithBody(s, PatchHandler)).Methods("POST", "OPTIONS")

	r.Handle("/startView/{view_slug}", handlers.New(s, StartViewHandler)).Methods("POST", "OPTIONS")

	r.Handle("/logs/{run_id}", handlers.SSE(s, LogsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/tasks/errors", handlers.New(s, GetTaskErrorsHandler)).Methods("GET", "OPTIONS")

	r.Handle("/tasks/create", handlers.WithBody(s, InitTaskHandler)).Methods("POST", "OPTIONS")
	r.Handle("/tasks/isSlugAvailable", handlers.New(s, IsTaskSlugAvailableHandler)).Methods("GET", "OPTIONS")
	r.Handle("/views/create", handlers.WithBody(s, InitViewHandler)).Methods("POST", "OPTIONS")
	r.Handle("/views/isSlugAvailable", handlers.New(s, IsViewSlugAvailableHandler)).Methods("GET", "OPTIONS")

	r.PathPrefix("/views").HandlerFunc(ProxyViewHandler(s.PortProxy)).Methods("GET", "POST", "OPTIONS")
	r.Handle("/dependencies/reinstall", handlers.New(s, ReinstallDependenciesHandler)).Methods("POST", "OPTIONS")

	// TODO: Remove this once the studio UI starts using studio operations.
	r.Handle("/autopilot/generate", handlers.WithBody(s, autopilot.GenerateHandler)).Methods("POST", "OPTIONS")
}

func GetVersionHandler(ctx context.Context, s *state.State, r *http.Request) (state.VersionMetadata, error) {
	return s.Version(ctx), nil
}

type StudioInfo struct {
	Workspace            string              `json:"workspace"`
	DefaultEnv           libapi.Env          `json:"defaultEnv"`
	InitialFallbackEnv   *libapi.Env         `json:"fallbackEnv"` // This is specifically the fallback env that the server starts with
	Host                 string              `json:"host"`
	IsSandbox            bool                `json:"isSandbox"`
	IsRebuilding         bool                `json:"isRebuilding"`
	OutdatedDependencies bool                `json:"outdatedDependencies"`
	HasDependencyError   bool                `json:"hasDependencyError"`
	ServerStatus         status.ServerStatus `json:"serverStatus"`
}

func GetInfoHandler(ctx context.Context, s *state.State, r *http.Request) (StudioInfo, error) {
	var fallbackEnv *libapi.Env
	if s.InitialRemoteEnvSlug != nil {
		env, err := s.GetEnv(ctx, *s.InitialRemoteEnvSlug)
		if err != nil {
			return StudioInfo{}, err
		}
		fallbackEnv = &env
	}

	var host string
	if s.LocalClient != nil {
		host = strings.Replace(s.LocalClient.Host(), "127.0.0.1", "localhost", 1)
	}

	if s.ServerHost != "" {
		host = s.ServerHost
	}

	info := StudioInfo{
		Workspace:          s.Dir,
		DefaultEnv:         env.NewLocalEnv(),
		InitialFallbackEnv: fallbackEnv,
		Host:               host,
		ServerStatus:       s.ServerStatus,
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
	Name    string                 `json:"name"`
	Slug    string                 `json:"slug"`
	Kind    EntityKind             `json:"kind"`
	Runtime buildtypes.TaskRuntime `json:"runtime"`
}

type ListEntrypointsHandlerResponse struct {
	Entrypoints       map[string][]EntityMetadata `json:"entrypoints"`
	RemoteEntrypoints []EntityMetadata            `json:"remoteEntrypoints"`
}

// ListEntrypointsHandler handles requests to the /dev/list endpoint. It generates a mapping from entrypoint relative to
// the dev server root to the list of tasks and views that use that entrypoint.
func ListEntrypointsHandler(ctx context.Context, state *state.State, r *http.Request) (ListEntrypointsHandlerResponse, error) {
	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	entrypoints := make(map[string][]EntityMetadata)
	remoteEntrypoints := make([]EntityMetadata, 0)

	for slug, taskConfig := range state.LocalTasks.Items() {
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

	for slug, viewConfig := range state.LocalViews.Items() {
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

	sortEntityMap(entrypoints)

	// List remote entrypoints if fallback env is specified
	if envSlug != nil {
		res, err := state.RemoteClient.ListTasks(ctx, *envSlug)
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
	Size     *int64           `json:"size"`
	Children []*FileNode      `json:"children"`
	Entities []EntityMetadata `json:"entities"`
}

func (f *FileNode) AddChild(child *FileNode) {
	f.Children = append(f.Children, child)
	slices.SortFunc(f.Children, func(a, b *FileNode) bool {
		return a.Path < b.Path
	})
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

type StartViewResponse struct {
	ViteServer string `json:"viteServer"`
}

// StartViewHandler handles requests to the /dev/tasks/<task_slug> endpoint.
func StartViewHandler(ctx context.Context, s *state.State, r *http.Request) (StartViewResponse, error) {
	vars := mux.Vars(r)
	viewSlug, ok := vars["view_slug"]
	if !ok {
		return StartViewResponse{}, libhttp.NewErrBadRequest("view slug was not supplied")
	}

	view, ok := s.LocalViews.Get(viewSlug)
	if !ok {
		return StartViewResponse{}, libhttp.NewErrBadRequest("view with slug %q not found", viewSlug)
	}

	vd, err := viewdir.NewViewDirectoryFromViewConfig(view.ViewConfig)
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
		for slug, cfg := range s.LocalViews.Items() {
			if slug == viewSlug {
				continue
			}

			if cfg.Root == view.ViewConfig.Root {
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
		port, err = views.FindVitePort()
		if err != nil {
			return StartViewResponse{}, err
		}

		serverURL = fmt.Sprintf("%s%s", s.ServerHost, libviews.BasePath(port, s.DevToken))

		// If a server host is specified, we send (Airplane) API requests to that host.
		client = api.NewClient(api.ClientOpts{
			Host: s.ServerHost,
		})
	}

	cmd, viteServer, closer, err := views.Dev(ctx, &vd, views.ViteOpts{
		Client:               client,
		TTY:                  false,
		RebundleDependencies: !depHashesEqual,
		UsesYarn:             usesYarn,
		Port:                 port,
		Token:                s.DevToken,
	})
	if err != nil {
		return StartViewResponse{}, err
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
		return libhttp.NewErrBadRequest("run id was not supplied")
	}

	run, err := state.GetRun(ctx, runID)
	if err != nil {
		return err
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
	Errors   []dev_errors.EntityError `json:"errors"`
	Warnings []dev_errors.EntityError `json:"warnings"`
	Info     []dev_errors.EntityError `json:"info"`
}

// GetTaskErrorsHandler returns any errors found for the task, grouped by level
func GetTaskErrorsHandler(ctx context.Context, state *state.State, r *http.Request) (GetTaskErrorResponse, error) {
	taskSlug := r.URL.Query().Get("slug")
	if taskSlug == "" {
		return GetTaskErrorResponse{}, errors.New("Task slug was not supplied, request path must be of the form /v0/tasks/warnings?slug=<task_slug>")
	}
	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	metadata, err := state.GetTaskErrors(ctx, taskSlug, pointers.ToString(envSlug))
	if err != nil {
		return GetTaskErrorResponse{}, err
	}
	info := []dev_errors.EntityError{}
	warnings := []dev_errors.EntityError{}
	errors := []dev_errors.EntityError{}

	for _, e := range metadata.Errors {
		if e.Level == dev_errors.LevelInfo {
			info = append(info, e)
		} else if e.Level == dev_errors.LevelWarning {
			warnings = append(warnings, e)
		} else if e.Level == dev_errors.LevelError {
			errors = append(errors, e)
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

// Sort entities-per-entrypoint for a consistent ordering.
func sortEntityMap(m map[string][]EntityMetadata) {
	for ep, entities := range m {
		slices.SortFunc(entities, func(a, b EntityMetadata) bool {
			if a.Slug == b.Slug {
				if a.Name == b.Name {
					return a.Kind < b.Kind
				}
				return a.Name < b.Name
			}
			return a.Slug < b.Slug
		})
		m[ep] = entities
	}
}
