//go:build !race

package apiext_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/airplanedev/cli/pkg/cli/dev"
	"github.com/airplanedev/cli/pkg/cli/dev/env"
	"github.com/airplanedev/cli/pkg/cli/dev/logs"
	"github.com/airplanedev/cli/pkg/cli/devconf"
	libresources "github.com/airplanedev/cli/pkg/cli/resources"
	"github.com/airplanedev/cli/pkg/cli/resources/kinds"
	"github.com/airplanedev/cli/pkg/cli/server"
	"github.com/airplanedev/cli/pkg/cli/server/apiext"
	"github.com/airplanedev/cli/pkg/cli/server/outputs"
	"github.com/airplanedev/cli/pkg/cli/server/state"
	"github.com/airplanedev/cli/pkg/cli/server/test_utils"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestExecute technically causes a "race condition" since `dev.Execute` executes in a separate goroutine, even though
// we don't use the result of the mock executor in that case.
func TestExecute(t *testing.T) {
	require := require.New(t)
	mockExecutor := new(dev.MockExecutor)
	slug := "my_task"

	taskDefinition := definitions.Definition{
		Name: "My Task",
		Slug: slug,
		Node: &definitions.NodeDefinition{
			Entrypoint:  "my_task.ts",
			NodeVersion: "18",
		},
		Parameters: []definitions.ParameterDefinition{
			{
				Slug: "param1",
				Type: "shorttext",
			},
			{
				Slug: "param2",
				Type: "shorttext",
			},
		},
	}
	taskDefinition.SetDefnFilePath("my_task.task.yaml")

	logBroker := logs.NewDevLogBroker()
	store := state.NewRunStore()
	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			RemoteClient: &api.MockClient{},
			Executor:     mockExecutor,
			Runs:         store,
			LocalTasks: state.NewStore(map[string]state.TaskState{
				slug: {
					TaskConfig: discover.TaskConfig{
						TaskID:         "tsk123",
						TaskRoot:       ".",
						TaskEntrypoint: "my_task.ts",
						Def:            taskDefinition,
						Source:         discover.ConfigSourceDefn,
					},
				},
			}),
			DevConfig: &devconf.DevConfig{},
		}, server.Options{}),
	)

	paramValues := api.Values{
		"param1": "a",
		"param2": "b",
	}

	runConfig := dev.LocalRunConfig{
		Name: "My Task",
		Kind: buildtypes.TaskKindNode,
		KindOptions: buildtypes.KindOptions{
			"entrypoint":  "my_task.ts",
			"nodeVersion": "18",
		},
		ParamValues:       paramValues,
		File:              "my_task.ts",
		Slug:              slug,
		ConfigAttachments: []libapi.ConfigAttachment{},
		RemoteClient:      &api.MockClient{},
		AliasToResource:   map[string]libresources.Resource{},
		LogBroker:         logBroker,
	}
	mockExecutor.On("Execute", mock.Anything, runConfig).Return(nil)
	body := h.POST("/v0/tasks/execute").
		WithJSON(apiext.ExecuteTaskRequest{
			Slug:        slug,
			ParamValues: paramValues,
		}).
		Expect().
		Status(http.StatusOK).Body()

	var resp dev.LocalRun
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	// TODO: Don't check prefix
	require.True(strings.HasPrefix(resp.RunID, utils.DevRunPrefix))

	run, found := store.Get(resp.RunID)
	require.True(found)
	require.Equal(paramValues, run.ParamValues)
	require.Equal(taskDefinition.Slug, run.TaskID)
	require.Equal(taskDefinition.Name, run.TaskName)
	require.False(run.IsStdAPI)
}

func TestExecuteFallback(t *testing.T) {
	require := require.New(t)
	mockExecutor := new(dev.MockExecutor)
	slug := "my_task"

	remoteEnv := libapi.Env{
		ID:   "env1234",
		Slug: "test",
		Name: "test",
	}

	store := state.NewRunStore()
	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			RemoteClient: &api.MockClient{
				Envs: map[string]libapi.Env{
					"test": remoteEnv,
				},
			},
			Executor:             mockExecutor,
			Runs:                 store,
			LocalTasks:           state.NewStore(map[string]state.TaskState{}),
			DevConfig:            &devconf.DevConfig{},
			InitialRemoteEnvSlug: pointers.String("test"),
		}, server.Options{}),
	)

	paramValues := api.Values{
		"param1": "a",
		"param2": "b",
	}
	body := h.POST("/v0/tasks/execute").
		WithJSON(apiext.ExecuteTaskRequest{
			Slug:        slug,
			ParamValues: paramValues,
		}).
		WithHeader("X-Airplane-Studio-Fallback-Env-Slug", "stage").
		Expect().
		Status(http.StatusOK).Body()

	var resp api.RunTaskResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	// TODO: Don't check prefix
	require.True(strings.HasPrefix(resp.RunID, "run"))

	run, found := store.Get(resp.RunID)
	require.True(found)
	require.False(run.IsStdAPI)
	require.Equal(run.EnvSlug, "test")
}

func TestExecuteDescendantFallback(t *testing.T) {
	require := require.New(t)
	mockExecutor := new(dev.MockExecutor)
	slug := "my_task"

	runID := "run1234"
	runstore := state.NewRunStore()
	runstore.Add("task1", runID, dev.LocalRun{
		Status:          api.RunSucceeded,
		RunID:           runID,
		ID:              runID,
		FallbackEnvSlug: "test",
	})

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			RemoteClient:         &api.MockClient{},
			Executor:             mockExecutor,
			Runs:                 runstore,
			LocalTasks:           state.NewStore(map[string]state.TaskState{}),
			DevConfig:            &devconf.DevConfig{},
			InitialRemoteEnvSlug: pointers.String("test"),
		}, server.Options{}),
	)

	paramValues := api.Values{
		"param1": "a",
		"param2": "b",
	}
	token, err := dev.GenerateInsecureAirplaneToken(dev.AirplaneTokenClaims{
		RunID: runID,
	})
	require.NoError(err)
	body := h.POST("/v0/tasks/execute").
		WithHeader("X-Airplane-Token", token).
		WithJSON(apiext.ExecuteTaskRequest{
			Slug:        slug,
			ParamValues: paramValues,
		}).
		Expect().
		Status(http.StatusOK).Body()

	var resp api.RunTaskResponse
	err = json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)

	run, found := runstore.Get(resp.RunID)
	require.True(found)
	require.True(run.Remote)
	require.False(run.IsStdAPI)
	require.Equal(run.EnvSlug, "test")
}

func TestExecuteBuiltin(t *testing.T) {
	require := require.New(t)
	mockExecutor := new(dev.MockExecutor)
	slug := "airplane:sql_query"

	taskDefinition := definitions.Definition{
		Name: "My Task",
		Slug: slug,
		Node: &definitions.NodeDefinition{
			Entrypoint:  "my_task.ts",
			NodeVersion: "18",
		},
		Resources: map[string]string{"my_db": "database"},
	}
	taskDefinition.SetDefnFilePath("my_task.task.yaml")

	logBroker := logs.NewDevLogBroker()
	store := state.NewRunStore()
	resourceID := utils.GenerateID(utils.DevResourcePrefix)
	dbResource := kinds.PostgresResource{
		BaseResource: libresources.BaseResource{
			Kind: kinds.ResourceKindPostgres,
			Slug: "database",
			ID:   resourceID,
		},
		Username: "postgres",
		Password: "password",
	}

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			RemoteClient: &api.MockClient{},
			Executor:     mockExecutor,
			Runs:         store,
			LocalTasks: state.NewStore(map[string]state.TaskState{
				slug: {
					TaskConfig: discover.TaskConfig{
						TaskID:         "tsk123",
						TaskRoot:       ".",
						TaskEntrypoint: "my_task.ts",
						Def:            taskDefinition,
						Source:         discover.ConfigSourceDefn,
					},
				},
			}),
			DevConfig: &devconf.DevConfig{Resources: map[string]env.ResourceWithEnv{
				"database": {
					Resource: &dbResource,
					Remote:   false,
				},
			}},
		}, server.Options{}))

	paramValues := api.Values{
		"query":           "select * from users limit 1",
		"transactionMode": "auto",
	}

	runConfig := dev.LocalRunConfig{
		ParamValues:  paramValues,
		Slug:         slug,
		RemoteClient: &api.MockClient{},
		AliasToResource: map[string]libresources.Resource{
			"db": &dbResource,
		},
		IsBuiltin: true,
		LogBroker: logBroker,
	}
	mockExecutor.On("Execute", mock.Anything, runConfig).Return(nil)

	body := h.POST("/v0/tasks/execute").
		WithJSON(apiext.ExecuteTaskRequest{
			Slug:        slug,
			ParamValues: paramValues,
			Resources:   map[string]string{"db": resourceID},
		}).
		Expect().
		Status(http.StatusOK).Body()

	var resp dev.LocalRun
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	// TODO: Don't check prefix
	require.True(strings.HasPrefix(resp.RunID, utils.DevRunPrefix))

	run, found := store.Get(resp.RunID)
	require.True(found)
	require.Equal(paramValues, run.ParamValues)
	require.Empty(run.TaskID)
	require.Equal(taskDefinition.Slug, run.TaskName)
	require.True(run.IsStdAPI)
}

func TestGetRun(t *testing.T) {
	require := require.New(t)

	runID := "run1234"
	runstore := state.NewRunStore()
	runstore.Add("task1", runID, dev.LocalRun{Status: api.RunSucceeded, RunID: runID, ID: runID})
	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			Runs:       runstore,
			LocalTasks: state.NewStore[string, state.TaskState](nil),
		}, server.Options{}),
	)
	body := h.GET("/v0/runs/get").
		WithQuery("id", runID).
		Expect().
		Status(http.StatusOK).Body()

	var resp dev.LocalRun
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)

	require.Equal(runID, resp.RunID)
	require.Equal(runID, resp.ID)
	require.Equal(api.RunSucceeded, resp.Status)
}

func TestGetOutput(t *testing.T) {
	require := require.New(t)
	runID := "run1234"

	runstore := state.NewRunStore()
	runstore.Add("task1", runID, dev.LocalRun{Outputs: api.Outputs{V: "hello"}})
	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			Runs:       runstore,
			LocalTasks: state.NewStore[string, state.TaskState](nil),
		}, server.Options{}),
	)

	body := h.GET("/v0/runs/getOutputs").
		WithQuery("id", runID).
		Expect().
		Status(http.StatusOK).Body()

	var resp outputs.GetOutputsResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(api.Outputs{
		V: "hello",
	}, resp.Output)
}

func TestRefresh(t *testing.T) {
	mockExecutor := new(dev.MockExecutor)
	slug := "my_task"

	taskDefinition := definitions.Definition{
		Name: "My Task",
		Slug: slug,
		Node: &definitions.NodeDefinition{
			Entrypoint:  "my_task.ts",
			NodeVersion: "18",
		},
	}
	taskDefinition.SetDefnFilePath("my_task.task.yaml")

	logBroker := logs.NewDevLogBroker()
	store := state.NewRunStore()
	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			RemoteClient: &api.MockClient{},
			Executor:     mockExecutor,
			Runs:         store,
			LocalTasks: state.NewStore(map[string]state.TaskState{
				slug: {
					TaskConfig: discover.TaskConfig{
						TaskID:         "tsk123",
						TaskRoot:       ".",
						TaskEntrypoint: "my_task.ts",
						Def:            taskDefinition,
						Source:         discover.ConfigSourceDefn,
					},
				},
			}),
			DevConfig: &devconf.DevConfig{},
		}, server.Options{}),
	)

	runConfig := dev.LocalRunConfig{
		Name: "My Task",
		Kind: buildtypes.TaskKindNode,
		KindOptions: buildtypes.KindOptions{
			"entrypoint":  "my_task.ts",
			"nodeVersion": "18",
		},
		File:              "my_task.ts",
		Slug:              slug,
		ParamValues:       map[string]interface{}{},
		ConfigAttachments: []libapi.ConfigAttachment{},
		RemoteClient:      &api.MockClient{},
		AliasToResource:   map[string]libresources.Resource{},
		LogBroker:         logBroker,
	}
	mockExecutor.On("Execute", mock.Anything, runConfig).Return(nil, &exec.ExitError{})
	mockExecutor.On("Refresh").Return(nil)
	mockExecutor.WG.Add(1)
	h.POST("/v0/tasks/execute").
		WithJSON(apiext.ExecuteTaskRequest{
			Slug: slug,
		}).
		Expect().
		Status(http.StatusOK).Body()

	done := make(chan struct{})
	go func() {
		mockExecutor.WG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}

	mockExecutor.AssertNumberOfCalls(t, "Refresh", 1)
}
