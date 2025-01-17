package initcmd

import (
	"testing"

	libapi "github.com/airplanedev/cli/pkg/api"
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/airplanedev/cli/pkg/testutils"
)

func TestInit(t *testing.T) {
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
		{
			Desc:       "JavaScript with folder",
			Inputs:     []interface{}{"My JavaScript task", "JavaScript", "folder/my_javascript_task.airplane.ts"},
			FixtureDir: "./fixtures/javascript_with_folder",
		},
		{
			Desc:       "Python with folder",
			Inputs:     []interface{}{"My Python task", "Python", "folder/my_python_task_airplane.py"},
			FixtureDir: "./fixtures/python_with_folder",
		},
		{
			Desc:       "SQL with folder",
			Inputs:     []interface{}{"My SQL task", "SQL", "folder/my_sql_task.sql", "folder/my_sql_task.task.yaml"},
			FixtureDir: "./fixtures/sql_with_folder",
		},
		{
			Desc:       "REST with folder",
			Inputs:     []interface{}{"My REST task", "REST", "folder/my_rest_task.task.yaml"},
			FixtureDir: "./fixtures/rest_with_folder",
		},
		{
			Desc:       "GraphQL with folder",
			Inputs:     []interface{}{"My GraphQL task", "GraphQL", "folder/my_graphql_task.task.yaml"},
			FixtureDir: "./fixtures/graphql_with_folder",
		},
		{
			Desc:       "Shell with folder",
			Inputs:     []interface{}{"My Shell task", "Shell", "folder/my_shell_task.sh", "folder/my_shell_task.task.yaml"},
			FixtureDir: "./fixtures/shell_with_folder",
		},
		{
			Desc:       "Docker with folder",
			Inputs:     []interface{}{"My Docker task", "Docker", "folder/my_docker_task.task.yaml"},
			FixtureDir: "./fixtures/docker_with_folder",
		},
		{
			Desc:       "Workflow with folder",
			Inputs:     []interface{}{"My workflow task", "JavaScript", "folder/my_workflow_task.airplane.ts"},
			FixtureDir: "./fixtures/workflow_with_folder",
			Args:       []string{"--workflow"},
		},
		{
			Desc:       "Noninline with folder",
			Inputs:     []interface{}{"Noninline", "JavaScript", "folder/noninline.ts", "folder/noninline.task.yaml"},
			FixtureDir: "./fixtures/noninline_with_folder",
			Args:       []string{"--inline=false"},
		},
		{
			Desc:   "Dry run JavaScript",
			Inputs: []interface{}{"My JavaScript task", "JavaScript", "my_javascript_task.airplane.ts"},
			Args:   []string{"--dry-run"},
		},
		{
			Desc:   "Dry run Python",
			Inputs: []interface{}{"My Python task", "Python", "my_python_task_airplane.py"},
			Args:   []string{"--dry-run"},
		},
		{
			Desc:   "Dry run SQL",
			Inputs: []interface{}{"My SQL task", "SQL", "my_sql_task.sql", "my_sql_task.task.yaml"},
			Args:   []string{"--dry-run"},
		},
		{
			Desc:   "Dry run REST",
			Inputs: []interface{}{"My REST task", "REST", "my_rest_task.task.yaml"},
			Args:   []string{"--dry-run"},
		},
		{
			Desc:   "Dry run GraphQL",
			Inputs: []interface{}{"My GraphQL task", "GraphQL", "my_graphql_task.task.yaml"},
			Args:   []string{"--dry-run"},
		},
		{
			Desc:   "Dry run Shell",
			Inputs: []interface{}{"My Shell task", "Shell", "my_shell_task.sh", "my_shell_task.task.yaml"},
			Args:   []string{"--dry-run"},
		},
		{
			Desc:   "Dry run Docker",
			Inputs: []interface{}{"My Docker task", "Docker", "my_docker_task.task.yaml"},
			Args:   []string{"--dry-run"},
		},
		{
			Desc:   "Dry run Workflow",
			Inputs: []interface{}{"My workflow task", "JavaScript", "my_workflow_task.airplane.ts"},
			Args:   []string{"--workflow", "--dry-run"},
		},
		{
			Desc:   "Dry run Noninline",
			Inputs: []interface{}{"Noninline", "JavaScript", "noninline.ts", "noninline.task.yaml"},
			Args:   []string{"--inline=false", "--dry-run"},
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
			var cfg = &cli.Config{
				Client:   mc,
				Prompter: prompts.NewMock(tC.Inputs...),
			}

			cmd := New(cfg)
			testutils.TestCommandAndCompare(t, cmd, tC.Args, tC.FixtureDir)
		})
	}
}
