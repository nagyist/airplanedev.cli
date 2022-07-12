package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
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
	mockExecutor := new(dev.MockExecutor)
	slug := "my_task"

	taskDefinition := &definitions.Definition_0_3{
		Name: "My Task",
		Slug: slug,
		Node: &definitions.NodeDefinition_0_3{
			Entrypoint:  "my_task.ts",
			NodeVersion: "18",
		},
	}
	taskDefinition.SetDefnFilePath("my_task.task.yaml")

	ctx := context.Background()
	h := getHttpExpect(
		context.Background(),
		t,
		newRouter(ctx, &State{
			envSlug:  "stage",
			executor: mockExecutor,
			port:     1234,
			runs:     map[string]LocalRun{},
			taskConfigs: map[string]discover.TaskConfig{
				slug: {
					TaskID:         "tsk123",
					TaskRoot:       ".",
					TaskEntrypoint: "my_task.ts",
					Def:            taskDefinition,
					Source:         discover.ConfigSourceDefn,
				},
			},
		}),
	)

	paramValues := api.Values{
		"param1": "a",
		"param2": "b",
	}

	runConfig := dev.LocalRunConfig{
		Name: "My Task",
		Kind: build.TaskKindNode,
		KindOptions: build.KindOptions{
			"entrypoint":  "my_task.ts",
			"nodeVersion": "18",
		},
		ParamValues: paramValues,
		Port:        1234,
		File:        "my_task.task.yaml",
		Slug:        slug,
		EnvSlug:     "stage",
	}
	mockExecutor.On("Execute", ctx, runConfig).Return(nil)

	body := h.POST("/v0/tasks/execute").
		WithJSON(ExecuteTaskRequest{
			Slug:        slug,
			ParamValues: paramValues,
		}).
		Expect().
		Status(http.StatusOK).Body()

	mockExecutor.AssertCalled(t, "Execute", ctx, runConfig)

	var resp ExecuteTaskResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.True(strings.HasPrefix(resp.RunID, "run"))
}

func TestGetRun(t *testing.T) {
	require := require.New(t)

	runID := "run1234"
	h := getHttpExpect(
		context.Background(),
		t,
		newRouter(context.Background(), &State{
			runs: map[string]LocalRun{
				runID: {
					status: api.RunSucceeded,
				},
			},
			taskConfigs: map[string]discover.TaskConfig{},
		}),
	)

	body := h.GET("/v0/runs/get").
		WithQuery("id", runID).
		Expect().
		Status(http.StatusOK).Body()

	var resp GetRunResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(runID, resp.ID)
	require.Equal(api.RunSucceeded, resp.Status)
}

func TestGetOutput(t *testing.T) {
	require := require.New(t)
	runID := "run1234"
	h := getHttpExpect(
		context.Background(),
		t,
		newRouter(context.Background(), &State{
			runs: map[string]LocalRun{
				runID: {
					outputs: api.Outputs{V: "hello"},
				},
			},
			taskConfigs: map[string]discover.TaskConfig{},
		}),
	)

	body := h.GET("/v0/runs/getOutputs").
		WithQuery("id", runID).
		Expect().
		Status(http.StatusOK).Body()

	var resp GetOutputsResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(api.Outputs{
		V: "hello",
	}, resp.Output)
}
