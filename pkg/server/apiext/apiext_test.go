//go:build !race

package apiext_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/dev/logs"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/server/apiext"
	"github.com/airplanedev/cli/pkg/server/dev_errors"
	"github.com/airplanedev/cli/pkg/server/outputs"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/server/test_utils"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	libresources "github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestExecute technically causes a "race condition" since `dev.Execute` executes in a separate goroutine, even though
// we don't use the result of the mock executor in that case.
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
		Parameters: []definitions.ParameterDefinition_0_3{
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
			TaskConfigs: state.NewStore[string, discover.TaskConfig](map[string]discover.TaskConfig{
				slug: {
					TaskID:         "tsk123",
					TaskRoot:       ".",
					TaskEntrypoint: "my_task.ts",
					Def:            taskDefinition,
					Source:         discover.ConfigSourceDefn,
				},
			}),
			DevConfig: &conf.DevConfig{},
		}, server.Options{}),
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

	store := state.NewRunStore()
	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			RemoteClient:   &api.MockClient{},
			Executor:       mockExecutor,
			Runs:           store,
			TaskConfigs:    state.NewStore[string, discover.TaskConfig](map[string]discover.TaskConfig{}),
			DevConfig:      &conf.DevConfig{},
			UseFallbackEnv: true,
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
}

func TestExecuteBuiltin(t *testing.T) {
	require := require.New(t)
	mockExecutor := new(dev.MockExecutor)
	slug := "airplane:sql_query"

	taskDefinition := &definitions.Definition_0_3{
		Name: "My Task",
		Slug: slug,
		Node: &definitions.NodeDefinition_0_3{
			Entrypoint:  "my_task.ts",
			NodeVersion: "18",
		},
		Resources: definitions.ResourceDefinition_0_3{Attachments: map[string]string{"my_db": "database"}},
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
			TaskConfigs: state.NewStore[string, discover.TaskConfig](map[string]discover.TaskConfig{
				slug: {
					TaskID:         "tsk123",
					TaskRoot:       ".",
					TaskEntrypoint: "my_task.ts",
					Def:            taskDefinition,
					Source:         discover.ConfigSourceDefn,
				},
			}),
			DevConfig: &conf.DevConfig{Resources: map[string]env.ResourceWithEnv{
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
			Runs:        runstore,
			TaskConfigs: state.NewStore[string, discover.TaskConfig](nil),
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
			Runs:        runstore,
			TaskConfigs: state.NewStore[string, discover.TaskConfig](nil),
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
	//require := require.New(t)
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

	logBroker := logs.NewDevLogBroker()
	store := state.NewRunStore()
	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			RemoteClient: &api.MockClient{},
			Executor:     mockExecutor,
			Runs:         store,
			TaskConfigs: state.NewStore[string, discover.TaskConfig](map[string]discover.TaskConfig{
				slug: {
					TaskID:         "tsk123",
					TaskRoot:       ".",
					TaskEntrypoint: "my_task.ts",
					Def:            taskDefinition,
					Source:         discover.ConfigSourceDefn,
				},
			}),
			DevConfig: &conf.DevConfig{},
		}, server.RouterOptions{}),
	)

	runConfig := dev.LocalRunConfig{
		Name: "My Task",
		Kind: build.TaskKindNode,
		KindOptions: build.KindOptions{
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
	mockExecutor.On("Execute", mock.Anything, runConfig).Return(nil, dev_errors.SignalKilled)
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
