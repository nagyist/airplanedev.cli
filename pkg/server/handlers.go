package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/pkg/errors"
)

type errorResponse struct {
	Error string `json:"error"`
}

// Wrap is a helper for implementing HTTP handlers. Any returned errors
// will be written to the HTTP response using WriteHTTPError.
func Wrap(f func(ctx context.Context, w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(r.Context(), w, r); err != nil {
			WriteHTTPError(w, r, err)
			return
		}
	}
}

// HandlerWithBody is an API handler that reads a JSON-encoded request body, calls a provided handler,
// and then writes the JSON encoded response. It is used for handling an API request with a body.
func HandlerWithBody[Req any, Resp any](state *State,
	f func(ctx context.Context, state *State, req Req) (Resp, error)) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		var req Req
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return errors.Wrap(err, "failed to decode request body")
		}

		resp, err := f(ctx, state, req)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(resp)
	})
}

// Handler is an API handler that calls a provided handler and then writes the JSON encoded response.
// It is used for handling an API request without a body.
func Handler[Resp any](state *State,
	f func(ctx context.Context, state *State, r *http.Request) (Resp, error)) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		resp, err := f(ctx, state, r)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(resp)
	})
}

// WriteHTTPError writes an error to response and optionally logs it
func WriteHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)

	// Render a JSON response.
	if err := json.NewEncoder(w).Encode(errorResponse{
		Error: err.Error(),
	}); err != nil {
		logger.Error(errors.Wrap(err, "encoding error response").Error())
		return
	}
}
