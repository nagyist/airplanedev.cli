package outputs

import (
	"context"
	"net/http"

	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/airplanedev/cli/pkg/cli/server/state"
	"github.com/pkg/errors"
)

type GetOutputsResponse struct {
	Output api.Outputs `json:"output"`
}

func GetOutputsHandler(ctx context.Context, state *state.State, r *http.Request) (GetOutputsResponse, error) {
	runID := r.URL.Query().Get("id")
	if runID == "" {
		runID = r.URL.Query().Get("runID")
	}
	run, err := state.GetRunInternal(ctx, runID)
	if err != nil {
		return GetOutputsResponse{}, err
	}
	outputs := run.Outputs

	if run.Remote {
		resp, err := state.RemoteClient.GetOutputs(ctx, runID)
		if err != nil {
			return GetOutputsResponse{}, errors.Wrap(err, "getting remote run outputs")
		}

		outputs = resp.Outputs
	}

	return GetOutputsResponse{
		Output: outputs,
	}, nil
}
