package apiint

import (
	"context"
	"net/http"

	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	libhttp "github.com/airplanedev/cli/pkg/cli/apiclient/http"
	"github.com/airplanedev/cli/pkg/cli/server/state"
)

type GetGroupResponse struct {
	Group libapi.Group `json:"group"`
}

func GetGroupHandler(ctx context.Context, state *state.State, r *http.Request) (GetGroupResponse, error) {
	groupID := r.URL.Query().Get("groupID")
	if groupID == "" {
		return GetGroupResponse{}, libhttp.NewErrBadRequest("groupID cannot be empty")
	}

	resp, err := state.RemoteClient.GetGroup(ctx, groupID)
	if err != nil {
		return GetGroupResponse{}, err
	}

	return GetGroupResponse{
		Group: resp.Group,
	}, nil
}
