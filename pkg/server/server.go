package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/resource"
	"github.com/airplanedev/cli/pkg/server/apidev"
	"github.com/airplanedev/cli/pkg/server/apiext"
	"github.com/airplanedev/cli/pkg/server/apiint"
	"github.com/airplanedev/cli/pkg/server/dev_errors"
	"github.com/airplanedev/cli/pkg/server/state"
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

// address returns the TCP address that the api server listens on
func address(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
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
		}),
	))

	apiext.AttachExternalAPIRoutes(r.NewRoute().Subrouter(), state)
	apiint.AttachInternalAPIRoutes(r.NewRoute().Subrouter(), state)
	apidev.AttachDevRoutes(r.NewRoute().Subrouter(), state)
	return r
}

type Options struct {
	LocalClient  *api.Client
	RemoteClient api.APIClient
	Env          api.Env
	Port         int
	Executor     dev.Executor
	DevConfig    *conf.DevConfig
	Dir          string
	AuthInfo     api.AuthInfoResponse
}

// newServer returns a new HTTP server with API routes
func newServer(router *mux.Router, state *state.State) *Server {
	srv := &http.Server{
		Addr:    address(state.Port),
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
	}

	viteContextCache, err := lrucache.NewWithEvict(5, onEvict)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vite context cache")
	}

	state := &state.State{
		RemoteClient: opts.RemoteClient,
		Env:          opts.Env,
		Executor:     opts.Executor,
		Port:         opts.Port,
		Runs:         state.NewRunStore(),
		LocalClient:  opts.LocalClient,
		DevConfig:    opts.DevConfig,
		ViteContexts: viteContextCache,
		Dir:          opts.Dir,
		Logger:       logger.NewStdErrLogger(logger.StdErrLoggerOpts{}),
		AuthInfo:     opts.AuthInfo,
	}

	r := NewRouter(state)
	s := newServer(r, state)

	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

// RegisterTasksAndViews generates a mapping of slug to task and view configs and stores the mappings in the server
// state. Task registration must occur after the local dev server has started because the task discoverer hits the
// /v0/tasks/getMetadata endpoint.
func (s *Server) RegisterTasksAndViews(ctx context.Context, taskConfigs []discover.TaskConfig, viewConfigs []discover.ViewConfig) (dev_errors.RegistrationWarnings, error) {
	s.state.TaskConfigs = map[string]discover.TaskConfig{}
	var unsupported []dev_errors.AppError
	var unattachedResources []dev_errors.UnattachedResource
	taskWarnings := map[string][]dev_errors.AppError{}
	mergedResources, err := resource.MergeRemoteResources(ctx, s.state)
	if err != nil {
		return dev_errors.RegistrationWarnings{}, errors.Wrap(err, "merging local and remote resources")
	}
	for _, cfg := range taskConfigs {
		kind, _, err := cfg.Def.GetKindAndOptions()
		if err != nil {
			return dev_errors.RegistrationWarnings{}, errors.Wrap(err, "getting task kind")
		}
		supported := supportsLocalExecution(cfg.Def.GetName(), cfg.TaskEntrypoint, kind)
		if !supported {
			unsupported = append(unsupported, dev_errors.AppError{
				Level:   dev_errors.LevelInfo,
				AppName: cfg.Def.GetName(),
				AppKind: apidev.AppKindTask,
				Reason:  fmt.Sprintf("%v task does not support local execution", kind)})
			continue
		}

		s.state.TaskConfigs[cfg.Def.GetSlug()] = cfg

		// Check resource attachments.
		var missingResources []string
		for _, resourceSlug := range cfg.Def.GetResourceAttachments() {
			if _, ok := mergedResources[resourceSlug]; !ok {
				missingResources = append(missingResources, resourceSlug)
			}
		}
		if len(missingResources) > 0 {
			unattachedResources = append(unattachedResources, dev_errors.UnattachedResource{
				TaskName:      cfg.Def.GetName(),
				ResourceSlugs: missingResources,
			})
			taskWarnings[cfg.Def.GetSlug()] = []dev_errors.AppError{
				{
					Level:   dev_errors.LevelWarning,
					AppName: cfg.Def.GetSlug(),
					AppKind: apidev.AppKindTask,
					Reason:  fmt.Sprintf("Attached resource: %v not found in dev config file or remotely.", missingResources),
				},
			}
		}
	}

	s.state.TaskErrors = taskWarnings
	s.state.ViewConfigs = map[string]discover.ViewConfig{}
	for _, cfg := range viewConfigs {
		s.state.ViewConfigs[cfg.Def.Slug] = cfg
	}

	return dev_errors.RegistrationWarnings{
		UnsupportedApps:     unsupported,
		UnattachedResources: unattachedResources,
	}, nil
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
