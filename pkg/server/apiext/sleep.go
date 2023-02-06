package apiext

import (
	"context"
	"net/http"
	"time"

	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	libhttp "github.com/airplanedev/lib/pkg/api/http"
	"github.com/pkg/errors"
)

type CreateSleepRequest struct {
	// DurationMs is the length of the sleep in milliseconds.
	DurationMs int `json:"durationMs"`
	// Until is a RFC3339 timestamp for the timer end.
	Until time.Time `json:"until"`
}

type CreateSleepResponse struct {
	ID string `json:"id"`
}

func CreateSleepHandler(ctx context.Context, state *state.State, r *http.Request, req CreateSleepRequest) (CreateSleepResponse, error) {
	runID, err := getRunIDFromToken(r)
	if err != nil {
		return CreateSleepResponse{}, err
	}
	if runID == "" {
		return CreateSleepResponse{}, errors.Errorf("expected runID from airplane token: %s", err.Error())
	}

	sleep := libapi.Sleep{
		ID:         utils.GenerateID("slp"),
		RunID:      runID,
		CreatedAt:  time.Now().UTC(),
		DurationMs: req.DurationMs,
		Until:      req.Until,
	}

	if _, err := state.Runs.Update(runID, func(run *dev.LocalRun) error {
		run.Sleeps = append(run.Sleeps, sleep)
		return nil
	}); err != nil {
		return CreateSleepResponse{}, err
	}

	return CreateSleepResponse{ID: sleep.ID}, nil
}

type GetSleepResponse struct {
	libapi.Sleep
}

func GetSleepHandler(ctx context.Context, state *state.State, r *http.Request) (GetSleepResponse, error) {
	sleepID := r.URL.Query().Get("id")
	if sleepID == "" {
		return GetSleepResponse{}, libhttp.NewErrBadRequest("id is required")
	}
	runID, err := getRunIDFromToken(r)
	if err != nil {
		return GetSleepResponse{}, err
	}
	if runID == "" {
		return GetSleepResponse{}, errors.Errorf("expected runID from airplane token")
	}

	run, ok := state.Runs.Get(runID)
	if !ok {
		return GetSleepResponse{}, libhttp.NewErrNotFound("run not found")
	}

	for _, s := range run.Sleeps {
		if s.ID == sleepID {
			return GetSleepResponse{Sleep: s}, nil
		}
	}

	return GetSleepResponse{}, libhttp.NewErrNotFound("sleep not found")
}

type ListSleepsResponse struct {
	Sleeps []libapi.Sleep `json:"sleeps"`
}

func ListSleepsHandler(ctx context.Context, state *state.State, r *http.Request) (ListSleepsResponse, error) {
	runID := r.URL.Query().Get("runID")
	if runID == "" {
		return ListSleepsResponse{}, libhttp.NewErrBadRequest("runID is required")
	}

	run, ok := state.Runs.Get(runID)
	if !ok {
		return ListSleepsResponse{}, libhttp.NewErrNotFound("run not found")

	}

	return ListSleepsResponse{Sleeps: run.Sleeps}, nil
}
