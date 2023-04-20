package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/server/apidev"
	"github.com/airplanedev/cli/pkg/server/apiext"
	"github.com/airplanedev/cli/pkg/server/apiint"
	"github.com/airplanedev/cli/pkg/server/dev_errors"
	"github.com/airplanedev/cli/pkg/server/filewatcher"
	"github.com/airplanedev/cli/pkg/server/middleware"
	"github.com/airplanedev/cli/pkg/server/network"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/server/status"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

type Server struct {
	srv      *http.Server
	listener net.Listener
	state    *state.State
}

const defaultPort = 4000

var corsOrigins = []string{
	`\.airplane\.so:5000$`,
	`\.airstage\.app$`,
	`\.airplane\.dev$`,
	`^http://localhost:`,
	`^http://127.0.0.1:`,
}

// NewRouter returns a new router for the local api server
func NewRouter(state *state.State, opts Options) *mux.Router {
	r := mux.NewRouter()
	r.Use(handlers.CORS(
		handlers.AllowCredentials(),
		handlers.AllowedOriginValidator(func(origin string) bool {
			for _, o := range corsOrigins {
				r := regexp.MustCompile(o)
				if r.MatchString(origin) {
					return true
				}
			}
			return false
		}),
		handlers.AllowedHeaders([]string{
			// All headers that are sent in API requests must be included here, except for those that
			// are allowed by default: https://developer.mozilla.org/en-US/docs/Glossary/CORS-safelisted_request_header
			"content-type", // Included so that we can send `application/json` requests.
			"accept-encoding",
			"idempotency-key",
			"x-team-id",
			"x-airplane-env-id",
			"x-airplane-env-slug",
			"x-airplane-token",
			"x-airplane-api-key",
			"x-airplane-view-token",
			"x-airplane-client-kind",
			"x-airplane-client-version",
			"x-airplane-client-source",
			"x-airplane-studio-fallback-env-slug",
			"x-airplane-dev-token",
			"x-airplane-sandbox-token",
		}),
		handlers.ExposedHeaders([]string{
			// All headers that are sent in API responses must be included here, except for those that
			// are exposed by default: https://developer.mozilla.org/en-US/docs/Glossary/CORS-safelisted_response_header
			"content-encoding",
			"idempotency-key",
			"x-airplane-retryable",
		}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
	))

	// Only validate token if the user is running the dev server in tunnel mode. In sandbox mode, the token is
	// validated upstream by the API server.
	if opts.Token != nil && !opts.Sandbox {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Requests to /dev/views are authenticated by the token in the path, since Vite does not support
				// appending query parameters or headers to source paths.
				if strings.HasPrefix(r.URL.Path, "/dev/views/") {
					if _, err := network.VerifyDevViewPath(r.URL.Path, opts.Token); err == nil {
						next.ServeHTTP(w, r)
						return
					}
				}

				if r.URL.Query().Get("__airplane_tunnel_token") == *opts.Token ||
					r.Header.Get("X-Airplane-Dev-Token") == *opts.Token {
					next.ServeHTTP(w, r)
					return
				}

				w.WriteHeader(http.StatusUnauthorized)
			})
		})
	}

	r.Use(middleware.ReqBodyDecompression)

	apiext.AttachExternalAPIRoutes(r.NewRoute().Subrouter(), state)
	apiint.AttachInternalAPIRoutes(r.NewRoute().Subrouter(), state)
	apidev.AttachDevRoutes(r.NewRoute().Subrouter(), state)
	return r
}

// Options are options when starting the local dev server.
type Options struct {
	// Port is the desired port to listen on. If 0, the first open port after 4000 will be used.
	Port int
	// Sandbox is used to configure sandbox-specific settings, such as binding the server to (0.0.0.0) so that it can be
	// accessed outside a container.
	Sandbox bool

	// Optional listener that will be used in lieu of port/expose configuration. This is used for ngrok tunnels.
	Listener net.Listener

	// Optional token that will install auth middleware for all non-OPTIONS requests. Auth will need to be passed
	// in the "Authorization" header with format "Bearer <token>".
	Token *string
}

// newServer returns a new HTTP server with API routes
func newServer(router *mux.Router, state *state.State, opts Options) (*Server, error) {
	srv := &http.Server{
		Handler: router,
	}

	if opts.Listener == nil {
		// If the port is 0, try to find an open port starting at 4000. Note that this is subject to a small race condition,
		// since we could potentially find an open port but not have it be available by the time we want to listen on it.
		// We cannot use net.Listen to find an open port since we want to check if the port is available on any network
		// interface (i.e. 0.0.0.0), but this causes a pop-up on macOS.
		var err error

		if opts.Port == 0 {
			var err error
			opts.Port, err = network.FindOpenPortFrom("", defaultPort, 100)
			if err != nil {
				return nil, err
			}
		} else if !network.IsPortOpen("", opts.Port) {
			return nil, errors.Errorf("port %d is already in use - select a different port or remove the --port flag to automatically find an open port", opts.Port)
		}

		addr := network.LocalAddress(opts.Port, opts.Sandbox)
		opts.Listener, err = net.Listen("tcp", addr)
		if err != nil {
			return nil, errors.Wrap(err, "listening on port")
		}
	}

	return &Server{
		srv:      srv,
		listener: opts.Listener,
		state:    state,
	}, nil
}

// Start starts and returns a new instance of the Airplane API server along with the port it is listening on.
func Start(opts Options) (*Server, int, error) {
	s, err := state.New(opts.Token)
	if err != nil {
		return nil, 0, err
	}

	r := NewRouter(s, opts)
	apiServer, err := newServer(r, s, opts)
	if err != nil {
		return nil, 0, err
	}

	go func() {
		if err := apiServer.srv.Serve(apiServer.listener); err != nil && err != http.ErrServerClosed {
			logger.Log("")
			logger.Error(fmt.Sprintf("failed to start api server: %v", err))
			os.Exit(1)
		}
	}()

	var port int
	tcpAddr, ok := apiServer.listener.Addr().(*net.TCPAddr)
	if ok {
		// If using an ngrok tunnel, the returned port will be invalid
		port = tcpAddr.Port
	}
	return apiServer, port, nil
}

// RegisterState updates the server's state with the given state.
func (s *Server) RegisterState(newState *state.State) {
	s.state.Flagger = newState.Flagger
	s.state.LocalClient = newState.LocalClient
	s.state.RemoteClient = newState.RemoteClient
	s.state.InitialRemoteEnvSlug = newState.InitialRemoteEnvSlug
	s.state.Executor = newState.Executor
	s.state.DevConfig = newState.DevConfig
	s.state.Dir = newState.Dir
	s.state.AuthInfo = newState.AuthInfo
	s.state.Discoverer = newState.Discoverer
	s.state.BundleDiscoverer = newState.BundleDiscoverer
	s.state.StudioURL = newState.StudioURL
	s.state.SandboxState = newState.SandboxState
	s.state.ServerHost = newState.ServerHost
}

func supportsLocalExecution(name string, entrypoint string, kind buildtypes.TaskKind) bool {
	r, err := runtime.Lookup(entrypoint, kind)
	if err != nil {
		logger.Debug("%s does not support local execution: %v", name, err)
		return false
	}
	// Check if task kind can be locally developed.
	return r.SupportsLocalExecution()
}

// ValidateTasks returns a map of slug -> AppError for unsupported tasks.
func ValidateTasks(ctx context.Context, taskConfigs []discover.TaskConfig) (map[string]dev_errors.AppError, error) {
	unsupportedApps := map[string]dev_errors.AppError{}

	for _, cfg := range taskConfigs {
		kind, _, err := cfg.Def.GetKindAndOptions()
		if err != nil {
			return nil, errors.Wrap(err, "getting task kind")
		}
		supported := supportsLocalExecution(cfg.Def.GetName(), cfg.TaskEntrypoint, kind)
		if !supported {
			appErr := dev_errors.AppError{
				Level:   dev_errors.LevelError,
				AppName: cfg.Def.GetName(),
				AppKind: apidev.EntityKindTask,
				Reason:  fmt.Sprintf("%v task does not support local execution", kind)}
			unsupportedApps[cfg.Def.GetSlug()] = appErr
			continue
		}
	}

	return unsupportedApps, nil
}

func (s *Server) DiscoverTasksAndViews(ctx context.Context, paths ...string) ([]discover.TaskConfig, []discover.ViewConfig, error) {
	if s.state.Discoverer == nil {
		return []discover.TaskConfig{}, []discover.ViewConfig{}, errors.New("discoverer not initialized")
	}
	taskConfigs, viewConfigs, err := s.state.Discoverer.Discover(ctx, paths...)
	if err != nil {
		return []discover.TaskConfig{}, []discover.ViewConfig{}, errors.Wrap(err, "discovering tasks and views")
	}

	return taskConfigs, viewConfigs, err
}

// shouldReloadDirectory returns whether the entire directory should be refreshed
// or an individual path
func shouldReloadDirectory(e filewatcher.Event) bool {
	// for deleted or moved events, we want to refresh the entire directory
	if e.Op == filewatcher.Remove || e.Op == filewatcher.Move {
		return true
	}
	return false
}

// ReloadApps takes in the changed file/directory and kicks off a
// goroutine to re-discover the task/view or reload the config file.
// It uses the state.Debouncer to debounce the actual refreshing.
func (s *Server) ReloadApps(ctx context.Context, wd string, e filewatcher.Event) error {
	shouldRefreshDir := shouldReloadDirectory(e)
	path := e.Path
	if shouldRefreshDir {
		path = wd
	}

	reload := func() {
		if path == s.state.DevConfig.Path {
			if err := s.state.DevConfig.Update(); err != nil {
				logger.Error("Loading dev config file: %s", err.Error())
			}
		}

		pathsToDiscover := []string{path}

		for _, tC := range s.state.TaskConfigs.Items() {
			var shouldRefreshTask bool
			// Refresh any tasks that have resource attachments
			if path == s.state.DevConfig.Path {
				ra, err := tC.Def.GetResourceAttachments()
				if err != nil {
					logger.Debug("Error getting resource attachments for task %s: %v", tC.Def.GetName(), err)
					continue
				}

				if len(ra) > 0 {
					shouldRefreshTask = true
				}
			}

			// Refresh any tasks that have the modified entrypoint.
			shouldRefreshTask = shouldRefreshTask || tC.TaskEntrypoint == path
			if shouldRefreshTask {
				pathsToDiscover = append(pathsToDiscover, tC.Def.GetDefnFilePath())
			}
		}

		// Refresh any views that have the modified entrypoint.
		for _, vC := range s.state.ViewConfigs.Items() {
			if vC.Def.Entrypoint == path {
				pathsToDiscover = append(pathsToDiscover, vC.Def.DefnFilePath)
			}
		}

		slices.Sort(pathsToDiscover)
		pathsToDiscover = utils.UniqueStrings(pathsToDiscover)

		taskConfigs, viewConfigs, err := s.DiscoverTasksAndViews(ctx, pathsToDiscover...)
		if err != nil {
			logger.Error(err.Error())
		}

		_, err = s.RegisterTasksAndViews(ctx, DiscoverOpts{
			Tasks:        taskConfigs,
			Views:        viewConfigs,
			OverwriteAll: shouldRefreshDir,
		})
		LogNewApps(taskConfigs, viewConfigs)
		if err != nil {
			logger.Error(err.Error())
		}
	}

	dfn, ok := s.state.Debouncers.Get(path)
	if !ok {
		dfn = utils.Debounce(time.Second, reload)
		s.state.Debouncers.Add(path, dfn)
	}
	// kick off a debounced version of the reload
	// debounce is non-blocking and will execute reload() in a separate goroutine
	dfn()

	return nil
}

// LogNewApps prints the names of the tasks/views that were discovered
func LogNewApps(tasks []discover.TaskConfig, views []discover.ViewConfig) {
	taskNames := make([]string, len(tasks))
	for i, task := range tasks {
		taskNames[i] = task.Def.GetName()
	}
	taskNoun := "tasks"
	if len(tasks) == 1 {
		taskNoun = "task"
	}
	time := time.Now().Format(logger.TimeFormatNoDate)
	if len(tasks) > 0 {
		logger.Log("%v Loaded %s: %v", logger.Yellow(time), taskNoun, strings.Join(taskNames, ", "))
	}
	viewNoun := "views"
	if len(views) == 1 {
		viewNoun = "view"
	}
	viewNames := make([]string, len(views))
	for i, view := range views {
		viewNames[i] = view.Def.Name
	}
	if len(views) > 0 {
		logger.Log("%v Loaded %s: %v", logger.Yellow(time), viewNoun, strings.Join(viewNames, ", "))
	}
}

type DiscoverOpts struct {
	Tasks []discover.TaskConfig
	Views []discover.ViewConfig
	// OverwriteAll will clear out existing tasks and views and replace them with the new ones
	OverwriteAll bool
}

// RegisterTasksAndViews generates a mapping of slug to task and view configs and stores the mappings in the server
// state. Task registration must occur after the local dev server has started because the task discoverer hits the
// /v0/tasks/getMetadata endpoint.
func (s *Server) RegisterTasksAndViews(ctx context.Context, opts DiscoverOpts) (map[string]dev_errors.AppError, error) {
	unsupportedApps, err := ValidateTasks(ctx, opts.Tasks)
	if err != nil {
		return nil, errors.Wrap(err, "validating task")
	}

	// Always invalidate the AppCondition cache.
	s.state.AppCondition.ReplaceItems(map[string]state.AppCondition{})

	taskConfigs := map[string]discover.TaskConfig{}
	for _, cfg := range opts.Tasks {
		if _, isUnsupported := unsupportedApps[cfg.Def.GetSlug()]; !isUnsupported {
			taskConfigs[cfg.Def.GetSlug()] = cfg
		}
	}
	viewConfigs := map[string]discover.ViewConfig{}
	for _, cfg := range opts.Views {
		viewConfigs[cfg.Def.Slug] = cfg
	}
	if opts.OverwriteAll {
		s.state.TaskConfigs.ReplaceItems(taskConfigs)
		s.state.ViewConfigs.ReplaceItems(viewConfigs)
	} else {
		s.state.TaskConfigs.AddMany(taskConfigs)
		s.state.ViewConfigs.AddMany(viewConfigs)
	}

	s.state.SetServerStatus(status.ServerReady)

	return unsupportedApps, err
}

// Stop terminates the local dev API server.
func (s *Server) Stop(ctx context.Context) error {
	s.state.ViteContexts.Purge()
	return s.srv.Shutdown(ctx)
}
