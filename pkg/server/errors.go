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
