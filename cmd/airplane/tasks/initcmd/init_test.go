package initcmd

import (
	"bytes"
	"testing"

	"github.com/airplanedev/cli/cmd/airplane/testutils"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/prompts"
	libapi "github.com/airplanedev/lib/pkg/api"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	deployconfig "github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/stretchr/testify/require"
)

func TestGetNewAirplaneConfig(t *testing.T) {
	testCases := []struct {
		desc              string
		cfg               deployconfig.AirplaneConfig
		existingConfig    deployconfig.AirplaneConfig
		hasExistingConfig bool
		newConfig         *deployconfig.AirplaneConfig
	}{
		{
			desc:      "Creates new empty config",
			newConfig: &deployconfig.AirplaneConfig{},
		},
		{
			desc: "Creates new config with node version and base",
			cfg: deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(buildtypes.BuildTypeVersionNode18),
					Base:        string(buildtypes.BuildBaseSlim),
				},
			},
			newConfig: &deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(buildtypes.BuildTypeVersionNode18),
					Base:        string(buildtypes.BuildBaseSlim),
				},
			},
		},
		{
			desc: "Does not update a non-empty config",
			cfg: deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(buildtypes.BuildTypeVersionNode18),
					Base:        string(buildtypes.BuildBaseSlim),
				},
			},
			hasExistingConfig: true,
			existingConfig: deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(buildtypes.BuildTypeVersionNode18),
					Base:        string(buildtypes.BuildBaseSlim),
				},
			},
		},
		{
			desc: "Updates existing, empty config",
			cfg: deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(buildtypes.BuildTypeVersionNode18),
				},
			},
			hasExistingConfig: true,
			newConfig: &deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(buildtypes.BuildTypeVersionNode18),
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			var w bytes.Buffer

			err := writeNewAirplaneConfig(&w, getNewAirplaneConfigOptions{
				cfg:               tC.cfg,
				existingConfig:    tC.existingConfig,
				hasExistingConfig: tC.hasExistingConfig,
			})
			require.NoError(t, err)

			c := &deployconfig.AirplaneConfig{}
			err = c.Unmarshal(w.Bytes())
			require.NoError(t, err)

			if w.Len() == 0 {
				require.Nil(t, tC.newConfig)
			} else {
				require.NotNil(t, tC.newConfig)
				require.Equal(t, *tC.newConfig, *c)
			}
		})
	}
}

func TestInit(t *testing.T) {
	// TODO(justin, 04152023): re-enable tests once we resolve the flaky yarn install collisions
	t.Skip()

	testCases := []testutils.InitTest{
		{
			Desc:       "JavaScript",
			Inputs:     []interface{}{"My JavaScript task", "JavaScript", "my_javascript_task.airplane.ts"},
			FixtureDir: "./fixtures/javascript",
		},
		{
			Desc:       "Python",
			Inputs:     []interface{}{"My Python task", "Python", "my_python_task_airplane.py"},
			FixtureDir: "./fixtures/python",
		},
		{
			Desc:       "SQL",
			Inputs:     []interface{}{"My SQL task", "SQL", "my_sql_task.sql", "my_sql_task.task.yaml"},
			FixtureDir: "./fixtures/sql",
		},
		{
			Desc:       "REST",
			Inputs:     []interface{}{"My REST task", "REST", "my_rest_task.task.yaml"},
			FixtureDir: "./fixtures/rest",
		},
		{
			Desc:       "GraphQL",
			Inputs:     []interface{}{"My GraphQL task", "GraphQL", "my_graphql_task.task.yaml"},
			FixtureDir: "./fixtures/graphql",
		},
		{
			Desc:       "Shell",
			Inputs:     []interface{}{"My Shell task", "Shell", "my_shell_task.sh", "my_shell_task.task.yaml"},
			FixtureDir: "./fixtures/shell",
		},
		{
			Desc:       "Docker",
			Inputs:     []interface{}{"My Docker task", "Docker", "my_docker_task.task.yaml"},
			FixtureDir: "./fixtures/docker",
		},
		{
			Desc:       "Workflow",
			Inputs:     []interface{}{"My workflow task", "JavaScript", "my_workflow_task.airplane.ts"},
			FixtureDir: "./fixtures/workflow",
			Args:       []string{"--workflow"},
		},
		{
			Desc:       "Noninline",
			Inputs:     []interface{}{"Noninline", "JavaScript", "noninline.ts", "noninline.task.yaml"},
			FixtureDir: "./fixtures/noninline",
			Args:       []string{"--inline=false"},
		},
		{
			Desc:       "From task",
			Inputs:     []interface{}{"sql_task.sql", "sql_task.task.yaml"},
			FixtureDir: "./fixtures/from",
			Args:       []string{"--from=sql_task"},
		},
		{
			Desc:       "From runbook",
			Inputs:     []interface{}{"existing_runbook.airplane.ts"},
			FixtureDir: "./fixtures/fromrunbook",
			Args:       []string{"--from-runbook=existing_runbook"},
		},
	}

	mc := api.NewMockClient()

	mc.Resources = []libapi.Resource{
		{
			ID:   "res0",
			Name: "DB",
			Slug: "db",
		},
		{
			ID:   "res1",
			Name: "REST",
			Slug: "rest",
		},
	}

	mc.Tasks = map[string]libapi.Task{
		"sql_task": {
			Name: "SQL task",
			Slug: "sql_task",
			Kind: buildtypes.TaskKindSQL,
			KindOptions: map[string]interface{}{
				"query":       "SELECT 1",
				"entrypoints": "sql_task.sql",
			},
		},
	}

	mc.Runbooks = map[string]api.Runbook{
		"existing_runbook": {
			ID:   "rbk1234",
			Name: "Existing runbook",
			Slug: "existing_runbook",
			TemplateSession: api.TemplateSession{
				ID: "ses1234",
			},
		},
	}

	mc.SessionBlocks = map[string][]api.SessionBlock{
		"ses1234": {
			{
				Slug:      "sql",
				BlockKind: "stdapi",
				BlockKindConfig: api.BlockKindConfig{
					StdAPI: &api.BlockKindConfigStdAPI{
						Namespace: "sql",
						Name:      "query",
						Request: map[string]interface{}{
							"query": "SELECT 1",
						},
						Resources: map[string]string{
							"db": "res0",
						},
					},
				},
			},
			{
				Slug:      "rest",
				BlockKind: "stdapi",
				BlockKindConfig: api.BlockKindConfig{
					StdAPI: &api.BlockKindConfigStdAPI{
						Namespace: "rest",
						Name:      "request",
						Request: map[string]interface{}{
							"method": "GET",
							"path":   "/",
						},
						Resources: map[string]string{
							"rest": "res1",
						},
					},
				},
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.Desc, func(t *testing.T) {
			subR := require.New(t)
			var cfg = &cli.Config{
				Client:   mc,
				Prompter: prompts.NewMock(tC.Inputs...),
			}

			cmd := New(cfg)
			testutils.TestCommandAndCompare(subR, cmd, tC.Args, tC.FixtureDir)
		})
	}
}
