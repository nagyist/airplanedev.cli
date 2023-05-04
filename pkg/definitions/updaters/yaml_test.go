package updaters

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/stretchr/testify/require"
)

func TestUpdateYAMLTask(t *testing.T) {
	testCases := []struct {
		name string
		slug string
		def  definitions.Definition
		json bool
	}{
		{
			// Tests setting various fields.
			name: "all",
			slug: "my_task",
			def: definitions.Definition{
				// This case also tests renaming a task slug.
				Slug:        "my_task_2",
				Name:        "Task name",
				Description: "Task description",
				Parameters: []definitions.ParameterDefinition{
					{
						Slug:        "dry",
						Name:        "Dry run?",
						Description: "Whether or not to run in dry-run mode.",
						Type:        "boolean",
						Required:    definitions.NewDefaultTrueDefinition(false),
						Default:     true,
					},
					{
						Slug:     "datetime",
						Type:     "datetime",
						Required: definitions.NewDefaultTrueDefinition(false),
					},
				},
				Runtime:            "workflow",
				RequireRequests:    true,
				AllowSelfApprovals: definitions.NewDefaultTrueDefinition(false),
				Timeout:            60,
				Constraints: map[string]string{
					"cluster": "k8s",
					"vpc":     "tasks",
				},
				Schedules: map[string]definitions.ScheduleDefinition{
					"all": {
						Name:        "All fields",
						Description: "A description",
						CronExpr:    "0 12 * * *",
						ParamValues: map[string]interface{}{
							"datetime": "2006-01-02T15:04:05Z07:00",
						},
					},
					"min": {
						CronExpr: "* * * * *",
					},
				},
				Resources: map[string]string{
					"db": "db",
				},
				ConcurrencyKey:   "scripts",
				ConcurrencyLimit: definitions.NewDefaultOneDefinition(5),
				Permissions: &definitions.PermissionsDefinition{
					Viewers:                    definitions.PermissionRecipients{Groups: []string{"group1"}, Users: []string{"user1"}},
					Requesters:                 definitions.PermissionRecipients{Groups: []string{"group2"}},
					Executers:                  definitions.PermissionRecipients{Groups: []string{"group3", "group4"}},
					Admins:                     definitions.PermissionRecipients{Groups: []string{"group5"}},
					RequireExplicitPermissions: true,
				},
				DefaultRunPermissions: definitions.NewDefaultTaskViewersDefinition(api.DefaultRunPermissionTaskParticipants),
			},
		},
		{
			// Tests the case where values are cleared.
			name: "all_cleared",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
			},
		},
		{
			// Tests the case where values are set to their default values (and therefore should not be serialized).
			name: "all_defaults",
			slug: "my_task",
			def: definitions.Definition{
				Slug:        "my_task",
				Name:        "",
				Description: "",
				Parameters:  []definitions.ParameterDefinition{},
				Resources:   map[string]string{},
				Node: &definitions.NodeDefinition{
					Entrypoint:  "./entrypoint.js",
					NodeVersion: "18",
					EnvVars:     api.EnvVars{},
				},
				Constraints:        map[string]string{},
				RequireRequests:    false,
				AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
				Timeout:            3600,
				Runtime:            types.TaskRuntimeStandard,
				Schedules:          map[string]definitions.ScheduleDefinition{}, ConcurrencyKey: "",
				ConcurrencyLimit: definitions.NewDefaultOneDefinition(1),
				Permissions: &definitions.PermissionsDefinition{
					RequireExplicitPermissions: false,
				},
				DefaultRunPermissions: definitions.NewDefaultTaskViewersDefinition(api.DefaultRunPermissionTaskViewers),
			},
		},
		{
			// Tests the case where a task uses all forms of parameters.
			name: "parameters",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Parameters: []definitions.ParameterDefinition{
					{
						Slug: "simple",
						Type: "shorttext",
					},
					{
						Name:        "All fields",
						Description: "My description",
						Type:        "shorttext",
						Required:    definitions.NewDefaultTrueDefinition(false),
						Slug:        "all",
						Default:     "My default",
						Regex:       "^.*$",
						Options: []definitions.OptionDefinition{
							{Value: "Thing 1"},
							{Value: "Thing 2"},
							{Label: "Thing 3", Value: "Secret gremlin"},
						},
					},
					{
						Slug:    "shorttext",
						Type:    "shorttext",
						Default: "Text",
					},
					{
						Slug:    "longtext",
						Type:    "longtext",
						Default: "Longer text",
					},
					{
						Slug:    "sql",
						Type:    "sql",
						Default: "SELECT 1",
					},
					{
						Slug:    "boolean_true",
						Type:    "boolean",
						Default: true,
					},
					{
						Slug:    "boolean_false",
						Type:    "boolean",
						Default: false,
					},
					{
						Slug:    "upload",
						Type:    "upload",
						Default: "upl123",
					},
					{
						Slug:    "integer",
						Type:    "integer",
						Default: 10,
					},
					{
						Slug:    "integer_zero",
						Type:    "integer",
						Default: 0,
					},
					{
						Slug:    "float",
						Type:    "float",
						Default: 3.14,
					},
					{
						Slug:    "float_zero",
						Type:    "float",
						Default: 0,
					},
					{
						Slug:    "date",
						Type:    "date",
						Default: "2006-01-02",
					},
					{
						Slug:    "datetime",
						Type:    "datetime",
						Default: "2006-01-02T15:04:05Z07:00",
					},
					{
						Slug:    "configvar",
						Type:    "configvar",
						Default: "MY_CONFIG",
					},
					{
						Slug: "configvar_legacy",
						Type: "configvar",
						// This is the legacy format for passing config vars as parameter values.
						Default: map[string]any{
							"config": "MY_CONFIG",
						},
					},
				},
			},
		},
		{
			// Tests the case where a resource has at least one alias. The "all" case checks for
			// the case where no aliases are used.
			name: "resource_aliases",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Resources: definitions.ResourcesDefinition{
					"my_alias": "alias",
					"no_alias": "no_alias",
				},
			},
		},
		{
			// Tests setting various fields and serializing as JSON.
			name: "json",
			json: true,
			slug: "my_task",
			def: definitions.Definition{
				// This case also tests renaming a task slug.
				Slug:        "my_task_2",
				Name:        "Task name",
				Description: "Task description",
				Parameters: []definitions.ParameterDefinition{
					{
						Slug:        "dry",
						Name:        "Dry run?",
						Description: "Whether or not to run in dry-run mode.",
						Type:        "boolean",
						Required:    definitions.NewDefaultTrueDefinition(false),
						Default:     true,
					},
					{
						Slug:     "datetime",
						Type:     "datetime",
						Required: definitions.NewDefaultTrueDefinition(false),
					},
				},
			},
		},
		{
			// Tests Node-specific task fields.
			name: "node",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Node: &definitions.NodeDefinition{
					Entrypoint:  "./entrypoint.js",
					NodeVersion: "18",
					EnvVars: api.EnvVars{
						"CONFIG": api.EnvVarValue{
							Config: pointers.String("aws_access_key"),
						},
						"VALUE": api.EnvVarValue{
							Value: pointers.String("Hello World!"),
						},
					},
				},
			},
		},
		{
			// Tests Python-specific task fields.
			name: "python",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Python: &definitions.PythonDefinition{
					Entrypoint: "./entrypoint.py",
					EnvVars: api.EnvVars{
						"CONFIG": api.EnvVarValue{
							Config: pointers.String("aws_access_key"),
						},
						"VALUE": api.EnvVarValue{
							Value: pointers.String("Hello World!"),
						},
					},
				},
			},
		},
		{
			// Tests Shell-specific task fields.
			name: "shell",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Shell: &definitions.ShellDefinition{
					Entrypoint: "./entrypoint.sh",
					EnvVars: api.EnvVars{
						"CONFIG": api.EnvVarValue{
							Config: pointers.String("aws_access_key"),
						},
						"VALUE": api.EnvVarValue{
							Value: pointers.String("Hello World!"),
						},
					},
				},
			},
		},
		{
			// Tests Image-specific task fields.
			name: "image",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Image: &definitions.ImageDefinition{
					Image:      "alpine:3",
					Command:    "cat",
					Entrypoint: "./entrypoint.sh",
					EnvVars: api.EnvVars{
						"CONFIG": api.EnvVarValue{
							Config: pointers.String("aws_access_key"),
						},
						"VALUE": api.EnvVarValue{
							Value: pointers.String("Hello World!"),
						},
					},
				},
			},
		},
		{
			// Tests SQL-specific task fields.
			name: "sql",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				SQL: &definitions.SQLDefinition{
					Resource:   "db",
					Entrypoint: "./entrypoint.sql",
					QueryArgs: map[string]interface{}{
						"test": "{{params.foo}}",
					},
					TransactionMode: "none",
					Configs:         []string{"DEFAULT_SQL_PAGE_SIZE"},
				},
			},
		},
		{
			// Tests SQL-specific task fields with defaults.
			name: "sql_defaults",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				SQL: &definitions.SQLDefinition{
					Resource:        "db",
					Entrypoint:      "./entrypoint.sql",
					QueryArgs:       map[string]interface{}{},
					TransactionMode: "auto",
					Configs:         []string{},
				},
			},
		},
		{
			// Tests REST-specific task fields.
			name: "rest",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				REST: &definitions.RESTDefinition{
					Resource: "api",
					Method:   "POST",
					Path:     "/events",
					URLParams: map[string]interface{}{
						"page": 10,
					},
					Headers: map[string]interface{}{
						"X-Foo": "bar",
					},
					BodyType: "json",
					Body:     `{"id": "ev123"}`,
					FormData: map[string]interface{}{
						"id": "ev123",
					},
					RetryFailures: true,
					Configs:       []string{"DEFAULT_PAGE_SIZE"},
				},
			},
		},
		{
			// Tests REST-specific task fields with defaults.
			name: "rest_defaults",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				REST: &definitions.RESTDefinition{
					Resource:      "api",
					Method:        "GET",
					Path:          "/",
					URLParams:     map[string]interface{}{},
					Headers:       map[string]interface{}{},
					BodyType:      "",
					Body:          nil,
					FormData:      map[string]interface{}{},
					RetryFailures: false,
					Configs:       []string{},
				},
			},
		},
		{
			// Tests GraphQL-specific task fields.
			name: "graphql",
			slug: "my_task",
			def: definitions.NewBuiltinDefinition("", "my_task", &definitions.GraphQLDefinition{
				Resource:  "api",
				Operation: "query GetPets {\n  pets {\n    name\n    petType\n  }\n}\n",
				Variables: map[string]interface{}{
					"id": "id123",
				},
				URLParams: map[string]interface{}{
					"page": 10,
				},
				Headers: map[string]interface{}{
					"X-Foo": "bar",
				},
				RetryFailures: true,
			}),
		},
		{
			// Tests GraphQL-specific task fields with defaults.
			name: "graphql_defaults",
			slug: "my_task",
			def: definitions.NewBuiltinDefinition("", "my_task", &definitions.GraphQLDefinition{
				Resource:      "api",
				Operation:     "query GetPets {\n  pets {\n    name\n    petType\n  }\n}\n",
				Variables:     map[string]interface{}{},
				URLParams:     map[string]interface{}{},
				Headers:       map[string]interface{}{},
				RetryFailures: false,
			}),
		},
	}
	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			require := require.New(t)
			l := logger.NewTestLogger(t)

			ext := "task.yaml"
			if tC.json {
				ext = "task.json"
			}

			// Clone the input file into a temporary directory as it will be overwritten by `Update()`.
			in, err := os.Open(fmt.Sprintf("./yaml/fixtures/%s.%s", tC.name, ext))
			require.NoError(err)
			f, err := os.CreateTemp("", "runtime-update-yaml-*."+ext)
			require.NoError(err)
			t.Cleanup(func() {
				require.NoError(os.Remove(f.Name()))
			})
			_, err = io.Copy(f, in)
			require.NoError(err)
			require.NoError(f.Close())

			canUpdate, err := CanUpdateYAMLTask(f.Name())
			require.NoError(err)
			require.True(canUpdate)

			// Perform the update on the temporary file.
			err = UpdateYAMLTask(context.Background(), l, f.Name(), tC.slug, tC.def)
			require.NoError(err)

			// Compare
			actual, err := os.ReadFile(f.Name())
			require.NoError(err)
			expected, err := os.ReadFile(fmt.Sprintf("./yaml/fixtures/%s.out.%s", tC.name, ext))
			require.NoError(err)
			require.Equal(string(expected), string(actual))
		})
	}
}
