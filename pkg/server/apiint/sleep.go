package apiint

import (
	"context"
	"net/http"
	"time"

	"github.com/airplanedev/cli/pkg/dev"
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

type SkipSleepRequest struct {
	SleepID string `json:"sleepID"`
	RunID   string `json:"runID"`
}

type SkipSleepResponse struct {
	ID string `json:"id"`
}

func SkipSleepHandler(ctx context.Context, state *state.State, r *http.Request, req SkipSleepRequest) (SkipSleepResponse, error) {
	if req.RunID == "" {
		return SkipSleepResponse{}, errors.New("runID is required")
	}
	if req.SleepID == "" {
		return SkipSleepResponse{}, errors.New("sleepID is required")
	}

	now := time.Now()

	if _, err := state.Runs.Update(req.RunID, func(run *dev.LocalRun) error {
		for i, sleep := range run.Sleeps {
			if sleep.ID == req.SleepID {
				run.Sleeps[i].SkippedAt = &now
				run.Sleeps[i].SkippedBy = &run.CreatorID
				return nil
			}
		}
		return nil
	}); err != nil {
		return SkipSleepResponse{}, err
	}

	return SkipSleepResponse{ID: req.SleepID}, nil
}
