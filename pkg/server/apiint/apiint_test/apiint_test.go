package apiint_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/server/apiext"
	"github.com/airplanedev/cli/pkg/server/apiint"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/server/test_utils"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/stretchr/testify/require"
)

// TODO: Add tests for other resource methods
func TestListResources(t *testing.T) {
	require := require.New(t)
	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			DevConfig: &conf.DevConfig{
				Resources: map[string]env.ResourceWithEnv{
					"db": {
						Resource: &kinds.PostgresResource{
							BaseResource: resources.BaseResource{
								ID:   "r-1",
								Slug: "db",
								Kind: kinds.ResourceKindPostgres,
							},
						},
						Remote: false,
					},
					"slack": {
						Resource: &kinds.SlackResource{
							BaseResource: resources.BaseResource{
								ID:   "r-2",
								Slug: "slack",
								Kind: kinds.ResourceKindSlack,
							},
						},
						Remote: false,
					},
				},
			},
			Env: env.NewLocalEnv(),
		}),
	)

	body := h.GET("/i/resources/list").
		Expect().
		Status(http.StatusOK).Body()

	var resp libapi.ListResourcesResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	expected := []libapi.Resource{
		{
			Slug: "db",
			ID:   "r-1",
			Kind: libapi.ResourceKind(kinds.ResourceKindPostgres),
		},
		{
			Slug: "slack",
			ID:   "r-2",
			Kind: libapi.ResourceKind(kinds.ResourceKindSlack),
		},
	}

	// sort so we can compare- since resources are stored as a map
	sort.Slice(resp.Resources, func(i, j int) bool {
		return resp.Resources[i].ID < resp.Resources[j].ID
	})

	for i := range expected {
		require.Equal(expected[i].Slug, resp.Resources[i].Slug)
		require.Equal(expected[i].ID, resp.Resources[i].ID)
		require.Equal(expected[i].Kind, resp.Resources[i].Kind)
	}
}

func TestSubmitPrompts(t *testing.T) {
	require := require.New(t)
	taskSlug := "task1"
	now := time.Now()
	runID := "run_0"
	promptID1 := utils.GenerateID("pmt")
	prompt1 := libapi.Prompt{
		ID:    promptID1,
		RunID: runID,
		Schema: libapi.Parameters{{
			Name: "option1",
			Slug: "option1",
			Type: "string",
		}},
		CreatedAt: now,
	}

	prompt2 := libapi.Prompt{
		ID:    "prompt2",
		RunID: runID,
		Schema: libapi.Parameters{{
			Name: "option2",
			Slug: "option2",
			Type: "boolean",
		}},
		CreatedAt: now,
	}

	runstore := state.NewRunStore()
	run := dev.LocalRun{
		RunID:   runID,
		TaskID:  taskSlug,
		Outputs: api.Outputs{V: "run0"},
		Prompts: []libapi.Prompt{prompt1, prompt2},
	}
	runstore.Add(taskSlug, runID, run)

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			Runs:        runstore,
			TaskConfigs: map[string]discover.TaskConfig{},
			DevConfig:   &conf.DevConfig{},
			CliConfig:   &cli.Config{Client: &api.Client{}},
		}),
	)

	var resp apiext.PromptResponse
	values := map[string]interface{}{"option1": "blah"}

	body := h.POST("/i/prompts/submit").
		WithJSON(apiint.SubmitPromptRequest{
			RunID:  runID,
			ID:     promptID1,
			Values: values,
		}).
		Expect().
		Status(http.StatusOK).Body()

	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(promptID1, resp.ID)

	body = h.GET("/i/prompts/list").
		WithQuery("runID", runID).
		Expect().
		Status(http.StatusOK).Body()

	listPrompts := apiint.ListPromptsResponse{}
	err = json.Unmarshal([]byte(body.Raw()), &listPrompts)
	require.NoError(err)
	require.Equal(2, len(listPrompts.Prompts))

	// check prompt 1 values match what was submitted
	require.Equal(values, listPrompts.Prompts[0].Values)
	require.NotNil(listPrompts.Prompts[0].SubmittedAt)
	require.NotNil(listPrompts.Prompts[0].SubmittedBy)

	// prompt 2 should be unsubmitted
	require.Equal(prompt2.ID, listPrompts.Prompts[1].ID)
	require.Nil(listPrompts.Prompts[1].Values)
	require.Nil(listPrompts.Prompts[1].SubmittedAt)
	require.Nil(listPrompts.Prompts[1].SubmittedBy)
}

func TestListRuns(t *testing.T) {
	require := require.New(t)
	taskSlug := "task1"

	runstore := state.NewRunStore()
	testRuns := []dev.LocalRun{
		{RunID: "run_0", TaskID: taskSlug, Outputs: api.Outputs{V: "run0"}},
		{RunID: "run_1", TaskID: taskSlug, Outputs: api.Outputs{V: "run1"}, CreatorID: "user1"},
		{RunID: "run_2", TaskID: taskSlug, Outputs: api.Outputs{V: "run2"}},
	}
	for i, run := range testRuns {
		runstore.Add(taskSlug, fmt.Sprintf("run_%v", i), run)

	}
	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			Runs:        runstore,
			TaskConfigs: map[string]discover.TaskConfig{},
		}),
	)
	var resp apiext.ListRunsResponse
	body := h.GET("/i/runs/list").
		WithQuery("taskSlug", taskSlug).
		Expect().
		Status(http.StatusOK).Body()
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)

	for i := range resp.Runs {
		// runHistory is ordered by most recent
		require.EqualValues(resp.Runs[i], testRuns[len(testRuns)-i-1])
	}
}
