package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/resources"
	"github.com/airplanedev/cli/pkg/server/apidev"
	"github.com/airplanedev/cli/pkg/server/apiext"
	"github.com/airplanedev/cli/pkg/server/apiint"
	"github.com/airplanedev/cli/pkg/server/dev_errors"
	"github.com/airplanedev/cli/pkg/server/filewatcher"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	lrucache "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
)

const DefaultPort = 4000

type Server struct {
	srv   *http.Server
	state *state.State
}

const containerAddress = "0.0.0.0"
const loopbackAddress = "127.0.0.1"

// address returns the TCP address that the API server listens on.
func address(port int, expose bool) string {
	var addr string
	if expose {
		addr = containerAddress
	} else {
		addr = loopbackAddress
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

var corsOrigins = []string{
	`\.airplane\.so:5000$`,
	`\.airstage\.app$`,
	`\.airplane\.dev$`,
	`^http://localhost:`,
	`^http://127.0.0.1:`,
}

// NewRouter returns a new router for the local api server
func NewRouter(state *state.State) *mux.Router {
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
			"content-type",
			"accept",
			"x-team-id",
			"x-airplane-env-id",
			"x-airplane-env-slug",
			"x-airplane-token",
			"x-airplane-api-key",
			"x-airplane-client-kind",
			"x-airplane-client-version",
			"x-airplane-client-source",
			"idempotency-key",
		}),
	))

	apiext.AttachExternalAPIRoutes(r.NewRoute().Subrouter(), state)
	apiint.AttachInternalAPIRoutes(r.NewRoute().Subrouter(), state)
	apidev.AttachDevRoutes(r.NewRoute().Subrouter(), state)
	return r
}

type Options struct {
	LocalClient    *api.Client
	RemoteClient   api.APIClient
	RemoteEnv      libapi.Env
	UseFallbackEnv bool

	Port int
	// Expose is used to bind the server to the default route (0.0.0.0) so that it can be accessed outside a container.
	Expose bool

	Executor   dev.Executor
	DevConfig  *conf.DevConfig
	Dir        string
	AuthInfo   api.AuthInfoResponse
	Discoverer *discover.Discoverer
}

// newServer returns a new HTTP server with API routes
func newServer(router *mux.Router, state *state.State, port int, expose bool) *Server {
	srv := &http.Server{
		Addr:    address(port, expose),
		Handler: router,
	}
	router.Handle("/shutdown", ShutdownHandler(srv))
	return &Server{
		srv:   srv,
		state: state,
	}
}

// Start starts and returns a new instance of the Airplane API server.
func Start(opts Options) (*Server, error) {
	onEvict := func(key interface{}, value interface{}) {
		viteContext, ok := value.(state.ViteContext)
		if !ok {
			logger.Error("expected vite context from context cache")
		}

		if err := viteContext.Process.Kill(); err != nil {
			logger.Error(fmt.Sprintf("could not shutdown existing vite process: %v", err))
		}

		if err := viteContext.Closer.Close(); err != nil {
			logger.Error(fmt.Sprintf("unable to cleanup vite process: %v", err))
		}
	}

	viteContextCache, err := lrucache.NewWithEvict(5, onEvict)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vite context cache")
	}

	state := &state.State{
		LocalClient:    opts.LocalClient,
		RemoteClient:   opts.RemoteClient,
		RemoteEnv:      opts.RemoteEnv,
		UseFallbackEnv: opts.UseFallbackEnv,
		Executor:       opts.Executor,
		Runs:           state.NewRunStore(),
		TaskConfigs:    state.NewStore[string, discover.TaskConfig](nil),
		AppCondition:   state.NewStore[string, state.AppCondition](nil),
		ViewConfigs:    state.NewStore[string, discover.ViewConfig](nil),
		Debouncer:      state.NewDebouncer(),
		DevConfig:      opts.DevConfig,
		ViteContexts:   viteContextCache,
		Dir:            opts.Dir,
		Logger:         logger.NewStdErrLogger(logger.StdErrLoggerOpts{}),
		AuthInfo:       opts.AuthInfo,
		Discoverer:     opts.Discoverer,
	}

	r := NewRouter(state)
	s := newServer(r, state, opts.Port, opts.Expose)

	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log("")
			logger.Error(fmt.Sprintf("failed to start api server: %v", err))
			os.Exit(1)
		}
	}()

	return s, nil
}

func supportsLocalExecution(name string, entrypoint string, kind build.TaskKind) bool {
	r, err := runtime.Lookup(entrypoint, kind)
	if err != nil {
		logger.Debug("%s does not support local execution: %v", name, err)
		return false
	}
	// Check if task kind can be locally developed.
	return r.SupportsLocalExecution()
}

// ValidateTasks returns any dev errors about tasks, such as whether local dev is supported
// and whether resources are attached
func ValidateTasks(ctx context.Context, resourcesWithEnv map[string]env.ResourceWithEnv, taskConfigs []discover.TaskConfig) (dev_errors.RegistrationWarnings, error) {
	unsupportedApps := map[string]dev_errors.AppError{}
	var unattachedResources []dev_errors.UnattachedResource
	taskErrors := map[string][]dev_errors.AppError{}

	for _, cfg := range taskConfigs {
		kind, _, err := cfg.Def.GetKindAndOptions()
		if err != nil {
			return dev_errors.RegistrationWarnings{}, errors.Wrap(err, "getting task kind")
		}
		supported := supportsLocalExecution(cfg.Def.GetName(), cfg.TaskEntrypoint, kind)
		if !supported {
			appErr := dev_errors.AppError{
				Level:   dev_errors.LevelError,
				AppName: cfg.Def.GetName(),
				AppKind: apidev.AppKindTask,
				Reason:  fmt.Sprintf("%v task does not support local execution", kind)}
			unsupportedApps[cfg.Def.GetSlug()] = appErr
			taskErrors[cfg.Def.GetSlug()] = []dev_errors.AppError{appErr}
			continue
		}

		// Check resource attachments.
		var missingResources []string
		resourceAttachments, err := cfg.Def.GetResourceAttachments()
		if err != nil {
			return dev_errors.RegistrationWarnings{}, errors.Wrap(err, "getting resource attachments")
		}
		for _, ref := range resourceAttachments {
			if _, ok := resources.LookupResource(resourcesWithEnv, ref); !ok {
				missingResources = append(missingResources, ref)
			}
		}
		if len(missingResources) > 0 {
			unattachedResources = append(unattachedResources, dev_errors.UnattachedResource{
				TaskName:      cfg.Def.GetName(),
				ResourceSlugs: missingResources,
			})
			taskErrors[cfg.Def.GetSlug()] = []dev_errors.AppError{
				{
					Level:   dev_errors.LevelWarning,
					AppName: cfg.Def.GetSlug(),
					AppKind: apidev.AppKindTask,
					Reason:  fmt.Sprintf("Attached resource: %v not found in dev config file or remotely.", missingResources),
				},
			}
		}
	}

	return dev_errors.RegistrationWarnings{
		UnsupportedApps:     unsupportedApps,
		UnattachedResources: unattachedResources,
		TaskErrors:          taskErrors,
	}, nil
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
func (s *Server) ReloadApps(ctx context.Context, path string, wd string, e filewatcher.Event) error {
	shouldRefreshDir := shouldReloadDirectory(e)
	if shouldRefreshDir {
		path = wd
	}
	var reload func()

	if path == s.state.DevConfig.Path {
		reload = func() {
			if err := s.state.DevConfig.LoadConfigFile(); err != nil {
				logger.Error("Loading dev config file: %s", err.Error())
			}
		}
	} else {
		reload = func() {
			pathsToDiscover := []string{path}
			// Refresh any tasks and views that have the modified entrypoint.
			for _, tC := range s.state.TaskConfigs.Items() {
				if tC.TaskEntrypoint == path {
					pathsToDiscover = append(pathsToDiscover, tC.Def.GetDefnFilePath())
				}
			}

			for _, vC := range s.state.ViewConfigs.Items() {
				if vC.Def.Entrypoint == path {
					pathsToDiscover = append(pathsToDiscover, vC.Def.DefnFilePath)
				}
			}
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
	}

	debounce := s.state.Debouncer.Get(path)
	// kick off a debounced version of the reload
	// debounce is non-blocking and will execute reload() in a separate goroutine
	debounce(reload)
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
func (s *Server) RegisterTasksAndViews(ctx context.Context, opts DiscoverOpts) (dev_errors.RegistrationWarnings, error) {
	mergedResources, err := resources.MergeRemoteResources(ctx, s.state)
	if err != nil {
		return dev_errors.RegistrationWarnings{}, errors.Wrap(err, "merging local and remote resources")
	}
	warnings, err := ValidateTasks(ctx, mergedResources, opts.Tasks)
	if err != nil {
		return dev_errors.RegistrationWarnings{}, errors.Wrap(err, "validating task")
	}
	if opts.OverwriteAll {
		// clear existing tasks, task errors, and views
		s.state.TaskConfigs.ReplaceItems(map[string]discover.TaskConfig{})
		s.state.AppCondition.ReplaceItems(map[string]state.AppCondition{})
		s.state.ViewConfigs.ReplaceItems(map[string]discover.ViewConfig{})
	}
	now := time.Now()
	for _, cfg := range opts.Tasks {
		if _, isUnsupported := warnings.UnsupportedApps[cfg.Def.GetSlug()]; !isUnsupported {
			s.state.TaskConfigs.Add(cfg.Def.GetSlug(), cfg)
		}
		w := warnings.TaskErrors[cfg.Def.GetSlug()]
		s.state.AppCondition.Add(cfg.Def.GetSlug(), state.AppCondition{RefreshedAt: now, Errors: w})
	}
	for _, cfg := range opts.Views {
		s.state.ViewConfigs.Add(cfg.Def.Slug, cfg)
	}
	return warnings, err
}

// Stop terminates the local dev API server.
func (s *Server) Stop(ctx context.Context) error {
	s.state.ViteContexts.Purge()
	return s.srv.Shutdown(ctx)
}

// ShutdownHandler manages shutdown requests. Shutdowns currently happen whenever the airplane dev logic has finished
// running, but in the future will be called when the user explicitly shuts down a long-running local dev api server.
func ShutdownHandler(s *http.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("OK")); err != nil {
			logger.Error("failed to write response for /shutdown")
		}
		// Call shutdown in a different goroutine so that the server can write a response first.
		go func() {
			if err := s.Shutdown(context.Background()); err != nil {
				panic(err)
			}
		}()
	}
}
