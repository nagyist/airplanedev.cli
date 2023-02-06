package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"unicode"

	"github.com/airplanedev/cli/pkg/server/state"
	libhttp "github.com/airplanedev/lib/pkg/api/http"
	"github.com/pkg/errors"
)

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

// WithBody is an API handler that reads a JSON-encoded request body, calls a provided handler,
// and then writes the JSON encoded response. It is used for handling an API request with a body.
func WithBody[Req any, Resp any](state *state.State,
	f func(ctx context.Context, state *state.State, r *http.Request, req Req) (Resp, error)) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		var req Req
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return errors.Wrap(err, "failed to decode request body")
		}

		resp, err := f(ctx, state, r, req)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(resp)
	})
}

// New is an API handler that calls a provided handler and then writes the JSON encoded response.
// It is used for handling an API request without a body.
func New[Resp any](state *state.State,
	f func(ctx context.Context, state *state.State, r *http.Request) (Resp, error)) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		resp, err := f(ctx, state, r)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(resp)
	})
}

func SSE[Resp any](state *state.State, f func(ctx context.Context, state *state.State, r *http.Request, flush func(resp Resp) error) error) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, _ := w.(http.Flusher)
		flush := func(resp Resp) error {
			if _, err := fmt.Fprint(w, "data: "); err != nil {
				return errors.Wrap(err, "serializing event field")
			}
			e := json.NewEncoder(w)
			e.SetEscapeHTML(false)
			if err := e.Encode(resp); err != nil {
				return errors.Wrap(err, "serializing event data")
			}
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return errors.Wrap(err, "serializing final event newline")
			}

			if flusher != nil {
				flusher.Flush()
			}

			return nil
		}

		err := f(ctx, state, r, flush)
		if err != nil {
			return err
		}

		return nil
	})
}

// Zip is an API handler used for returning zip files in HTTP responses.
func Zip(state *state.State,
	f func(ctx context.Context, state *state.State, r *http.Request) ([]byte, string, error)) http.HandlerFunc {
	return Wrap(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		contents, name, err := f(ctx, state, r)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", name))
		if _, err := w.Write(contents); err != nil {
			return err
		}

		return nil
	})
}

// WriteHTTPError writes an error to response and optionally logs it
func WriteHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()

	errStatusCode, retryable := GetErrorStatus(ctx, err)

	// TODO: Log 500 errors to sentry

	out, err := json.Marshal(libhttp.ErrorResponse{
		Error: errStatusCode.Msg,
	})
	if err != nil {
		// TODO: Report to sentry
		return
	}
	// This extra newline is for consistency with how we previously use an Encoder to write JSON responses.
	out = append(out, '\n')

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(out)))
	w.Header().Set("X-Airplane-Retryable", strconv.FormatBool(retryable))
	w.WriteHeader(errStatusCode.StatusCode)

	if _, err := w.Write(out); err != nil {
		// TODO: Report to sentry
		return
	}
}

// GetErrorStatus gets an errors HTTP status, code and message.
func GetErrorStatus(ctx context.Context, err error) (errStatusCode libhttp.ErrStatusCode, retryable bool) {
	switch {
	case errors.As(err, &errStatusCode):
	default:
		if ctx.Err() != nil {
			// If the cancellation stems from the http request, then the
			// client closed the connection to the server.
			errStatusCode = libhttp.NewErrBadRequest("Client closed request")
		} else {
			errStatusCode = libhttp.NewErrInternalServerError(fmt.Sprintf("An internal error occurred: %s", err.Error()))
		}
	}

	// Ensure that the error message is capitalized (more user friendly), as Go errors are conventionally
	// lowercase (for error wrapping).
	if len(errStatusCode.Msg) > 0 {
		r := []rune(errStatusCode.Msg)
		r[0] = unicode.ToUpper(r[0])
		errStatusCode.Msg = string(r)
	}

	return errStatusCode, retryable
}
