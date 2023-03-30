package apiext

import (
	"context"
	"net/http"

	"github.com/airplanedev/cli/pkg/api/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/api/http"
	"github.com/airplanedev/cli/pkg/server/state"
)

type StubResponse struct{}

type ExecuteRunbookRequest struct {
	Slug        string     `json:"slug"`
	ParamValues api.Values `json:"paramValues"`
}

type ExecuteRunbookResponse struct{}

func ExecuteRunbookHandler(
	ctx context.Context,
	state *state.State,
	r *http.Request,
	req CreateRunnerScaleSignalRequest,
) (StubResponse, error) {
	return StubResponse{}, libhttp.NewErrNotImplemented("runbooks are not supported in studio")
}

func GetRunbooksHandler(ctx context.Context, state *state.State, r *http.Request) (StubResponse, error) {
	return StubResponse{}, libhttp.NewErrNotImplemented("runbooks are not supported in studio")
}
