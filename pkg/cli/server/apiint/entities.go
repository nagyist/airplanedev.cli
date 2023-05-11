package apiint

import (
	"context"
	"net/http"

	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/airplanedev/cli/pkg/cli/server/state"
)

type SearchEntitiesResponse struct {
	Results []libapi.EntitySearchResult `json:"results"`
}

func SearchEntitiesHandler(ctx context.Context, state *state.State, r *http.Request) (SearchEntitiesResponse, error) {
	query := r.URL.Query().Get("q")
	scope := r.URL.Query().Get("scope")

	apiResp, err := state.RemoteClient.SearchEntities(ctx, libapi.EntitySearchScope(scope), query)
	if err != nil {
		return SearchEntitiesResponse{}, err
	}
	return SearchEntitiesResponse{Results: apiResp.Results}, nil
}
