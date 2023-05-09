package apiint

import (
	"context"
	"net/http"
	"time"

	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	libhttp "github.com/airplanedev/cli/pkg/cli/apiclient/http"
	"github.com/airplanedev/cli/pkg/cli/dev"
	"github.com/airplanedev/cli/pkg/cli/server/state"
)

type ListSleepsResponse struct {
	Sleeps []libapi.Sleep `json:"sleeps"`
}

func ListSleepsHandler(ctx context.Context, state *state.State, r *http.Request) (ListSleepsResponse, error) {
	runID := r.URL.Query().Get("runID")
	if runID == "" {
		return ListSleepsResponse{}, libhttp.NewErrBadRequest("runID is required")
	}

	run, err := state.GetRunInternal(ctx, runID)
	if err != nil {
		return ListSleepsResponse{}, err
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
		return SkipSleepResponse{}, libhttp.NewErrBadRequest("runID is required")
	}
	if req.SleepID == "" {
		return SkipSleepResponse{}, libhttp.NewErrBadRequest("sleepID is required")
	}

	now := time.Now()

	if _, err := state.UpdateRun(req.RunID, func(run *dev.LocalRun) error {
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
