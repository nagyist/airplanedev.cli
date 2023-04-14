package apiint

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/airplanedev/cli/pkg/schedules"
	"github.com/airplanedev/cli/pkg/server/state"
)

type ValidateCronExprRequest = struct {
	CronExpr schedules.CronExpr `json:"cronExpr"`
}

type ValidateCronExprResponse = struct {
	ErrorMsg          string      `json:"errorMsg"`
	NextScheduledRuns []time.Time `json:"nextScheduledRuns"`
}

func ValidateCronExprHandler(
	ctx context.Context,
	state *state.State,
	r *http.Request,
	req ValidateCronExprRequest,
) (ValidateCronExprResponse, error) {
	if err := req.CronExpr.Validate(); err != nil {
		return ValidateCronExprResponse{
			ErrorMsg: fmt.Sprintf("Invalid cron expression: %s", err.Error()),
		}, nil
	}

	return ValidateCronExprResponse{
		NextScheduledRuns: req.CronExpr.NextN(3),
	}, nil
}
