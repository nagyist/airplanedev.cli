package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/airplanedev/ojson"
	"github.com/gavv/httpexpect/v2"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func getHttpExpect(ctx context.Context, t *testing.T, router *mux.Router) *httpexpect.Expect {
	return httpexpect.WithConfig(httpexpect.Config{
		Reporter: httpexpect.NewAssertReporter(t),
		Client: &http.Client{
			Transport: httpexpect.NewBinder(router),
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Jar: httpexpect.NewJar(),
		},
		Context: ctx,
	})
}

func TestExecute(t *testing.T) {
	require := require.New(t)
	h := getHttpExpect(
		context.Background(),
		t,
		newRouter(),
	)

	body := h.POST("/v0/tasks/execute").
		WithJSON(map[string]interface{}{}).
		Expect().
		Status(http.StatusOK).Body()

	var resp ExecuteTaskResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.True(strings.HasPrefix(resp.RunID, "run"))
}

func TestGetRun(t *testing.T) {
	require := require.New(t)
	h := getHttpExpect(
		context.Background(),
		t,
		newRouter(),
	)

	runID := "run1234"
	body := h.GET("/v0/runs/get").
		WithQuery("id", runID).
		Expect().
		Status(http.StatusOK).Body()

	var resp GetRunResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(runID, resp.ID)
}

func TestGetOutput(t *testing.T) {
	require := require.New(t)
	h := getHttpExpect(
		context.Background(),
		t,
		newRouter(),
	)

	body := h.GET("/v0/runs/getOutputs").
		Expect().
		Status(http.StatusOK).Body()

	var resp GetOutputsResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(ojson.Value{
		V: nil,
	}, resp.Output)
}
