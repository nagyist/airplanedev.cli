package apiint

import (
	"context"
	"net/http"

	"github.com/airplanedev/cli/pkg/server/state"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/pkg/errors"
)

type ListSleepsResponse struct {
	Sleeps []libapi.Sleep `json:"sleeps"`
}

func ListSleepsHandler(ctx context.Context, state *state.State, r *http.Request) (ListSleepsResponse, error) {
	runID := r.URL.Query().Get("runID")
	if runID == "" {
		return ListSleepsResponse{}, errors.New("runID is required")
	}

	run, ok := state.Runs.Get(runID)
	if !ok {
		return ListSleepsResponse{}, errors.New("run not found")
	}

	return ListSleepsResponse{Sleeps: run.Sleeps}, nil
}
