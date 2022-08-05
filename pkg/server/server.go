package server

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

const DefaultPort = 4000

type Server struct {
	srv   *http.Server
	state *State
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
}

// newRouter returns a new router for the local api server
func newRouter(state *State) *mux.Router {
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
		}),
	))

	AttachAPIRoutes(r.NewRoute().Subrouter(), state)
	AttachDevRoutes(r.NewRoute().Subrouter(), state)
	return r
}

type Options struct {
	CLI       *cli.Config
	EnvSlug   string
	Port      int
	Executor  dev.Executor
	DevConfig conf.DevConfig
}

// newServer returns a new HTTP server with API routes
func newServer(router *mux.Router, state *State) *Server {
	srv := &http.Server{
		Addr:    address(state.port),
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
	state := &State{
		cliConfig: opts.CLI,
		envSlug:   opts.EnvSlug,
		executor:  opts.Executor,
		port:      opts.Port,
		runs:      NewRunStore(),
		devConfig: opts.DevConfig,
	}

	r := newRouter(state)
	s := newServer(r, state)

	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("failed to start api server")
		}
	}()

	return s, nil
}

// RegisterTasksAndViews generates a mapping of slug to task and view configs and stores the mappings in the server
// state. Task registration must occur after the local dev server has started because the task discoverer hits the
// /v0/tasks/getMetadata endpoint.
func (s *Server) RegisterTasksAndViews(taskConfigs []discover.TaskConfig, viewConfigs []discover.ViewConfig) {
	s.state.taskConfigs = map[string]discover.TaskConfig{}
	for _, cfg := range taskConfigs {
		s.state.taskConfigs[cfg.Def.GetSlug()] = cfg
	}

	s.state.viewConfigs = map[string]discover.ViewConfig{}
	for _, cfg := range viewConfigs {
		s.state.viewConfigs[cfg.Def.Slug] = cfg
	}
}

// Stop terminates the local dev API server.
func (s *Server) Stop(ctx context.Context) error {
	if s.state.viteProcess != nil {
		if err := s.state.viteProcess.Kill(); err != nil {
			return err
		}
	}

	if err := s.srv.Shutdown(ctx); err != nil {
		return err
	}
	return nil
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
