package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/airplanedev/cli/pkg/conf"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/stretchr/testify/require"
)

func TestListResources(t *testing.T) {
	require := require.New(t)
	h := getHttpExpect(
		context.Background(),
		t,
		newRouter(&State{
			devConfig: &conf.DevConfig{
				Resources: map[string]resources.Resource{
					"db": kinds.PostgresResource{
						BaseResource: resources.BaseResource{
							Slug: "db",
						},
					},
					"slack": kinds.SlackResource{
						BaseResource: resources.BaseResource{
							Slug: "slack",
						},
					},
				},
			},
		}),
	)

	body := h.GET("/i/resources/list").
		Expect().
		Status(http.StatusOK).Body()

	var resp libapi.ListResourcesResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.ElementsMatch([]libapi.Resource{
		{
			Slug: "db",
		},
		{
			Slug: "slack",
		},
	}, resp.Resources)
}
