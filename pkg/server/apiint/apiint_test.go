package apiint_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/resources"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/server/apiext"
	"github.com/airplanedev/cli/pkg/server/apiint"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/server/test_utils"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	libresources "github.com/airplanedev/lib/pkg/resources"
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
							BaseResource: libresources.BaseResource{
								ID:   "r-1",
								Slug: "db",
								Kind: kinds.ResourceKindPostgres,
							},
						},
						Remote: false,
					},
					"slack": {
						Resource: &kinds.SlackResource{
							BaseResource: libresources.BaseResource{
								ID:   "r-2",
								Slug: "slack",
								Kind: kinds.ResourceKindSlack,
							},
						},
						Remote: false,
					},
				},
			},
			RemoteClient: &api.MockClient{
				Resources: []libapi.Resource{
					{
						ID:             "res0",
						Slug:           resources.DemoDBSlug,
						Kind:           libapi.ResourceKind(kinds.ResourceKindPostgres),
						ExportResource: &kinds.PostgresResource{},
					},
				},
			},
		}),
	)

	body := h.GET("/i/resources/list").
		Expect().
		Status(http.StatusOK).Body()

	var resp apiint.ListResourcesResponse
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
		{
			Slug: resources.DemoDBSlug,
			ID:   "res0",
			Kind: libapi.ResourceKind(kinds.ResourceKindPostgres),
		},
	}

	require.Equal(len(expected), len(resp.Resources))

	// sort so we can compare, since resources are stored as a map
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
			Runs:         runstore,
			TaskConfigs:  state.NewStore[string, discover.TaskConfig](nil),
			DevConfig:    &conf.DevConfig{},
			RemoteClient: &api.MockClient{},
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
			TaskConfigs: state.NewStore[string, discover.TaskConfig](nil),
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

func TestGetUser(t *testing.T) {
	require := require.New(t)
	avatarURL := "https://www.gravatar.com/avatar?d=mp"
	user := api.User{
		ID:        "usr1234",
		Email:     "test@airplane.dev",
		Name:      "Air Plane",
		AvatarURL: &avatarURL,
	}
	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			RemoteClient: &api.MockClient{
				Users: map[string]api.User{"usr1234": user},
			},
		}),
	)
	var resp api.GetUserResponse
	body := h.GET("/i/users/get").
		WithQuery("userID", "usr1234").
		Expect().
		Status(http.StatusOK).Body()
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(user, resp.User)

	// Nonexistent return user id should return default user
	body = h.GET("/i/users/get").
		WithQuery("userID", "usr2345").
		Expect().
		Status(http.StatusOK).Body()
	err = json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(apiint.DefaultUser("usr2345"), resp.User)
}

func TestConfigsCRUD(t *testing.T) {
	require := require.New(t)

	dir, err := os.MkdirTemp("", "cli_test")
	require.NoError(err)
	path := filepath.Join(dir, "airplane.dev.yaml")

	cfg0 := env.ConfigWithEnv{
		Config: api.Config{
			ID:       utils.GenerateID(utils.DevConfigPrefix),
			Name:     "cv_0",
			Value:    "v0",
			IsSecret: false,
		},
		Remote: false,
		Env:    env.NewLocalEnv(),
	}

	cfg1 := env.ConfigWithEnv{
		Config: api.Config{
			ID:       utils.GenerateID(utils.DevConfigPrefix),
			Name:     "cv_1",
			Value:    "v1",
			IsSecret: false,
		},
		Remote: false,
		Env:    env.NewLocalEnv(),
	}

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			DevConfig: &conf.DevConfig{
				ConfigVars: map[string]env.ConfigWithEnv{
					"cv_0": cfg0,
					"cv_1": cfg1,
				},
				Path: path,
			},
		}),
	)

	// Test listing
	var listResp apiint.ListConfigsResponse
	body := h.GET("/i/configs/list").
		Expect().
		Status(http.StatusOK).Body()
	err = json.Unmarshal([]byte(body.Raw()), &listResp)
	require.NoError(err)
	// sort so we can compare, since resources are stored as a map
	expected := []env.ConfigWithEnv{cfg0, cfg1}
	sort.Slice(expected, func(i, j int) bool {
		return expected[i].ID < expected[j].ID
	})
	sort.Slice(listResp.Configs, func(i, j int) bool {
		return listResp.Configs[i].ID < listResp.Configs[j].ID
	})
	require.Equal(expected, listResp.Configs)

	// Test getting
	var getResp apiint.GetConfigResponse
	body = h.GET("/i/configs/get").
		WithQuery("id", cfg0.ID).
		Expect().
		Status(http.StatusOK).Body()
	err = json.Unmarshal([]byte(body.Raw()), &getResp)
	require.NoError(err)
	require.Equal(cfg0, getResp.Config)

	// Test update
	//nolint: staticcheck
	body = h.POST("/i/configs/upsert").
		WithJSON(apiint.UpsertConfigRequest{Name: cfg0.Name, Value: "v2"}).
		Expect().
		Status(http.StatusOK).Body()

	var getResp2 apiint.GetConfigResponse
	body = h.GET("/i/configs/get").
		WithQuery("id", cfg0.ID).
		Expect().
		Status(http.StatusOK).Body()
	err = json.Unmarshal([]byte(body.Raw()), &getResp2)
	require.NoError(err)
	require.Equal(env.ConfigWithEnv{
		Config: api.Config{
			ID:       cfg0.ID,
			Name:     cfg0.Name,
			Value:    "v2",
			IsSecret: cfg0.IsSecret,
		},
		Remote: cfg0.Remote,
		Env:    cfg0.Env,
	}, getResp2.Config)

	// Test deleting
	//nolint: staticcheck
	body = h.POST("/i/configs/delete").
		WithJSON(apiint.DeleteConfigRequest{ID: cfg0.ID}).
		Expect().
		Status(http.StatusOK).Body()

	var listResp2 apiint.ListConfigsResponse
	body = h.GET("/i/configs/list").
		Expect().
		Status(http.StatusOK).Body()
	err = json.Unmarshal([]byte(body.Raw()), &listResp2)
	require.NoError(err)
	require.Equal([]env.ConfigWithEnv{cfg1}, listResp2.Configs)
}

func TestRemoteConfigs(t *testing.T) {
	require := require.New(t)

	cfg0 := env.ConfigWithEnv{
		Config: api.Config{
			ID:       utils.GenerateID(utils.DevConfigPrefix),
			Name:     "cv_0",
			Value:    "v0",
			IsSecret: false,
		},
		Remote: false,
		Env:    env.NewLocalEnv(),
	}

	cfg1 := env.ConfigWithEnv{
		Config: api.Config{
			ID:       utils.GenerateID(utils.DevConfigPrefix),
			Name:     "cv_1",
			Value:    "v1",
			IsSecret: false,
		},
		Remote: false,
		Env:    env.NewLocalEnv(),
	}

	remoteCfg := api.Config{
		ID:       "cfg1234",
		Name:     "remote_config",
		Tag:      "",
		Value:    "test",
		IsSecret: false,
	}

	remoteEnv := libapi.Env{
		ID:   "env1234",
		Slug: "test",
		Name: "test",
	}

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			DevConfig: &conf.DevConfig{
				ConfigVars: map[string]env.ConfigWithEnv{
					"cv_0": cfg0,
					"cv_1": cfg1,
				},
			},
			RemoteClient: &api.MockClient{
				Configs: []api.Config{remoteCfg},
			},
			RemoteEnv:      remoteEnv,
			UseFallbackEnv: true,
		}),
	)

	// Test listing
	var listResp apiint.ListConfigsResponse
	body := h.GET("/i/configs/list").
		Expect().
		Status(http.StatusOK).Body()
	err := json.Unmarshal([]byte(body.Raw()), &listResp)
	require.NoError(err)
	// sort so we can compare, since resources are stored as a map
	expected := []env.ConfigWithEnv{cfg0, cfg1, {
		Config: remoteCfg,
		Remote: true,
		Env:    remoteEnv,
	}}
	sort.Slice(expected, func(i, j int) bool {
		return expected[i].ID < expected[j].ID
	})
	sort.Slice(listResp.Configs, func(i, j int) bool {
		return listResp.Configs[i].ID < listResp.Configs[j].ID
	})
	require.Equal(expected, listResp.Configs)
}