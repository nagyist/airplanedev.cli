package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/gorilla/mux"
)

const DefaultPort = 7190

// address returns the TCP address that the api server listens on
func address(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// newRouter returns a new router for the local api server
func newRouter() *mux.Router {
	r := mux.NewRouter()
	AttachAPIRoutes(r.NewRoute().Subrouter())
	return r
}

// newServer returns a new HTTP server with API routes
func newServer(router *mux.Router, port int) *http.Server {
	s := &http.Server{
		Addr:    address(port),
		Handler: router,
	}
	router.Handle("/shutdown", ShutdownHandler(s))
	return s
}

// Start starts a new instance of the Airplane API server for local dev.
func Start(port int) error {
	r := newRouter()
	s := newServer(r, port)

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("failed to start api server")
		}
	}()

	return nil
}

// Stop terminates the local dev API server.
func Stop(port int) error {
	if _, err := http.Get(fmt.Sprintf("http://%s/shutdown", address(port))); err != nil {
		return err
	}

	return nil
}

// ShutdownHandler manages shutdown requests. Shutdowns currently happen whenever the airplane dev logic has finished
// running, but in the future will be called when the user explicitly shuts down a long-running local dev api server.
// As such, we shutdown through a network request instead of
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
