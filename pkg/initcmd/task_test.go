package initcmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	libapi "github.com/airplanedev/cli/pkg/api"
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	deployconfig "github.com/airplanedev/cli/pkg/deploy/config"
	"github.com/airplanedev/cli/pkg/logger"
	_ "github.com/airplanedev/cli/pkg/runtime/javascript"
	_ "github.com/airplanedev/cli/pkg/runtime/python"
	_ "github.com/airplanedev/cli/pkg/runtime/rest"
	_ "github.com/airplanedev/cli/pkg/runtime/shell"
	_ "github.com/airplanedev/cli/pkg/runtime/sql"
	_ "github.com/airplanedev/cli/pkg/runtime/typescript"
	"github.com/airplanedev/cli/pkg/testutils"
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
				logger:            logger.NewNoopLogger(),
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

func TestInitTask(t *testing.T) {
	initcmdDir := "../../cmd/airplane/tasks/initcmd"
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
				"query": "SELECT 1",
			},
		},
	}

	testCases := []struct {
		desc       string
		req        InitTaskRequest
		setup      func(*testing.T, string)
		fixtureDir string
		hasError   bool
	}{
		{
			desc:     "no info",
			hasError: true,
		},
		{
			desc: "JavaScript",
			req: InitTaskRequest{
				Client:   mc,
				File:     "my_javascript_task.airplane.ts",
				Inline:   true,
				TaskName: "My JavaScript task",
				TaskKind: buildtypes.TaskKindNode,
			},
			fixtureDir: initcmdDir + "/fixtures/javascript",
		},
		{
			desc: "JavaScript without file",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				TaskName: "My JavaScript task",
				TaskKind: buildtypes.TaskKindNode,
			},
			fixtureDir: initcmdDir + "/fixtures/javascript",
		},
		{
			desc: "Python",
			req: InitTaskRequest{
				Client:   mc,
				File:     "my_python_task_airplane.py",
				Inline:   true,
				TaskName: "My Python task",
				TaskKind: buildtypes.TaskKindPython,
			},
			fixtureDir: initcmdDir + "/fixtures/python",
		},
		{
			desc: "Python without file",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				TaskName: "My Python task",
				TaskKind: buildtypes.TaskKindPython,
			},
			fixtureDir: initcmdDir + "/fixtures/python",
		},
		{
			desc: "SQL",
			req: InitTaskRequest{
				Client:   mc,
				File:     "my_sql_task.task.yaml",
				Inline:   true,
				TaskName: "My SQL task",
				TaskKind: buildtypes.TaskKindSQL,
			},
			fixtureDir: initcmdDir + "/fixtures/sql",
		},
		{
			desc: "SQL without file",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				TaskName: "My SQL task",
				TaskKind: buildtypes.TaskKindSQL,
			},
			fixtureDir: initcmdDir + "/fixtures/sql",
		},
		{
			desc: "REST",
			req: InitTaskRequest{
				Client:   mc,
				File:     "my_rest_task.task.yaml",
				Inline:   true,
				TaskName: "My REST task",
				TaskKind: buildtypes.TaskKindREST,
			},
			fixtureDir: initcmdDir + "/fixtures/rest",
		},
		{
			desc: "REST without file",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				TaskName: "My REST task",
				TaskKind: buildtypes.TaskKindREST,
			},
			fixtureDir: initcmdDir + "/fixtures/rest",
		},
		{
			desc: "GraphQL",
			req: InitTaskRequest{
				Client:       mc,
				File:         "my_graphql_task.task.yaml",
				Inline:       true,
				TaskName:     "My GraphQL task",
				TaskKind:     buildtypes.TaskKindBuiltin,
				TaskKindName: "GraphQL",
			},
			fixtureDir: initcmdDir + "/fixtures/graphql",
		},
		{
			desc: "GraphQL without file",
			req: InitTaskRequest{
				Client:       mc,
				Inline:       true,
				TaskName:     "My GraphQL task",
				TaskKind:     buildtypes.TaskKindBuiltin,
				TaskKindName: "GraphQL",
			},
			fixtureDir: initcmdDir + "/fixtures/graphql",
		},
		{
			desc: "Shell",
			req: InitTaskRequest{
				Client:   mc,
				File:     "my_shell_task.task.yaml",
				Inline:   true,
				TaskName: "My Shell task",
				TaskKind: buildtypes.TaskKindShell,
			},
			fixtureDir: initcmdDir + "/fixtures/shell",
		},
		{
			desc: "Shell without file",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				TaskName: "My Shell task",
				TaskKind: buildtypes.TaskKindShell,
			},
			fixtureDir: initcmdDir + "/fixtures/shell",
		},
		{
			desc: "Docker",
			req: InitTaskRequest{
				Client:   mc,
				File:     "my_docker_task.task.yaml",
				Inline:   true,
				TaskName: "My Docker task",
				TaskKind: buildtypes.TaskKindImage,
			},
			fixtureDir: initcmdDir + "/fixtures/docker",
		},
		{
			desc: "Docker without file",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				TaskName: "My Docker task",
				TaskKind: buildtypes.TaskKindImage,
			},
			fixtureDir: initcmdDir + "/fixtures/docker",
		},
		{
			desc: "Workflow",
			req: InitTaskRequest{
				Client:   mc,
				File:     "my_workflow_task.airplane.ts",
				Inline:   true,
				Workflow: true,
				TaskName: "My workflow task",
				TaskKind: buildtypes.TaskKindNode,
			},
			fixtureDir: initcmdDir + "/fixtures/workflow",
		},
		{
			desc: "Workflow without file",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				Workflow: true,
				TaskName: "My workflow task",
				TaskKind: buildtypes.TaskKindNode,
			},
			fixtureDir: initcmdDir + "/fixtures/workflow",
		},
		{
			desc: "Noninline",
			req: InitTaskRequest{
				Client:   mc,
				File:     "noninline.ts",
				TaskName: "Noninline",
				TaskKind: buildtypes.TaskKindNode,
			},
			fixtureDir: initcmdDir + "/fixtures/noninline",
		},
		{
			desc: "Noninline without file",
			req: InitTaskRequest{
				Client:   mc,
				TaskName: "Noninline",
				TaskKind: buildtypes.TaskKindNode,
			},
			fixtureDir: initcmdDir + "/fixtures/noninline",
		},
		{
			desc: "From task",
			req: InitTaskRequest{
				Client:   mc,
				FromTask: "sql_task",
			},
			fixtureDir: initcmdDir + "/fixtures/from",
		},
		{
			desc: "Existing JavaScript entrypoint",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				TaskName: "My JavaScript task",
				TaskKind: buildtypes.TaskKindNode,
			},
			setup: func(t *testing.T, wd string) {
				err := os.WriteFile(filepath.Join(wd, "my_javascript_task.airplane.ts"), []byte{}, 0655)
				require.NoError(t, err)
			},
			fixtureDir: "./fixtures/javascript_with_entrypoint",
		},
		{
			desc: "Existing package.json",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				TaskName: "My JavaScript task",
				TaskKind: buildtypes.TaskKindNode,
			},
			setup: func(t *testing.T, wd string) {
				err := os.WriteFile(filepath.Join(wd, "package.json"), []byte(`{"name": "javascript", "version": "1.0.5", "license": "MIT", "description": "foo"}`), 0655)
				require.NoError(t, err)
			},
			fixtureDir: initcmdDir + "/fixtures/javascript",
		},
		{
			desc: "Existing Python entrypoint",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				TaskName: "My Python task",
				TaskKind: buildtypes.TaskKindPython,
			},
			setup: func(t *testing.T, wd string) {
				err := os.WriteFile(filepath.Join(wd, "my_python_task_airplane.py"), []byte{}, 0655)
				require.NoError(t, err)
			},
			fixtureDir: "./fixtures/python_with_entrypoint",
		},
		{
			desc: "Existing defn file",
			req: InitTaskRequest{
				Client:   mc,
				Inline:   true,
				TaskName: "My REST task",
				TaskKind: buildtypes.TaskKindREST,
			},
			setup: func(t *testing.T, wd string) {
				err := os.WriteFile(filepath.Join(wd, "my_rest_task.task.yaml"), []byte{}, 0655)
				require.NoError(t, err)
			},
			fixtureDir: "./fixtures/rest_with_defn",
		},
		{
			desc: "Existing defn file, exceed cap",
			req: InitTaskRequest{
				Client:        mc,
				Inline:        true,
				TaskName:      "My REST task",
				TaskKind:      buildtypes.TaskKindREST,
				suffixCharset: "a",
			},
			setup: func(t *testing.T, wd string) {
				err := os.WriteFile(filepath.Join(wd, "my_rest_task.task.yaml"), []byte{}, 0655)
				require.NoError(t, err)

				for i := 1; i < 10; i++ {
					err = os.WriteFile(filepath.Join(wd, fmt.Sprintf("my_rest_task_%d.task.yaml", i)), []byte{}, 0655)
					require.NoError(t, err)
				}

				err = os.WriteFile(filepath.Join(wd, "my_rest_task_aaa.task.yaml"), []byte{}, 0655)
				require.NoError(t, err)
			},
			hasError: true,
		},
	}
	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			require := require.New(t)
			ctx := context.Background()

			testutils.TestWithWorkingDirectory(t, test.fixtureDir, func(wd string) bool {
				if test.setup != nil {
					test.setup(t, wd)
				}

				test.req.WorkingDirectory = wd

				_, err := InitTask(ctx, test.req)
				if test.hasError {
					require.Error(err)
					return false
				} else {
					require.NoError(err)
					return true
				}
			})
		})
	}
}
