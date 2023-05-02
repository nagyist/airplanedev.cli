package updaters

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/stretchr/testify/require"
)

func TestUpdateJavaScriptTask(t *testing.T) {
	testCases := []struct {
		name string
		slug string
		def  definitions.Definition
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
				RestrictCallers:    []string{"view", "task"},
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
				Node: &definitions.NodeDefinition{
					EnvVars: api.TaskEnv{
						"CONFIG": api.EnvVarValue{
							Config: pointers.String("aws_access_key"),
						},
						"VALUE": api.EnvVarValue{
							Value: pointers.String("Hello World!"),
						},
					},
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
					EnvVars: api.TaskEnv{},
				},
				Constraints:        map[string]string{},
				RequireRequests:    false,
				AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
				RestrictCallers:    []string{},
				Timeout:            3600,
				Runtime:            buildtypes.TaskRuntimeStandard,
				Schedules:          map[string]definitions.ScheduleDefinition{},
				ConcurrencyKey:     "",
				ConcurrencyLimit:   definitions.NewDefaultOneDefinition(1),
				Permissions: &definitions.PermissionsDefinition{
					RequireExplicitPermissions: false,
				},
			},
		},
		{
			// Tests edge cases for object keys and (string) values:
			// 1. The slug's key is a string literal ("slug") instead of an identifier (slug).
			// 2. A field is updated that is a valid identifier (it should not be wrapped in quotes).
			// 3. A field is updated that is not a valid identifier (it should be wrapped in quotes).
			// 4. A field is updated that includes single quotes.
			// 5. A field is updated that includes double quotes.
			// 6. A field is updated that includes both quotes.
			name: "keys",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Constraints: map[string]string{
					"a_valid_identifier":    "...",
					"an invalid identifier": "...",
					"double\"":              "\"",
					"single'":               "'",
					"both'\"'\"":            "'\"'\"",
				},
			},
		},
		{
			// Tests the case a file contains multiple tasks.
			name: "multiple_tasks",
			slug: "my_task_2",
			def: definitions.Definition{
				Slug: "my_task_two",
				Name: "My task (v2)",
			},
		},
		{
			// Tests the case where a task uses tabs.
			name: "tabs",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Name: "Task name",
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
			// Tests the case where the file contains invalid JS.
			name: "invalid_code",
			slug: "my_task",
			def: definitions.Definition{
				Slug:        "my_task",
				Description: "Added a description!",
			},
		},
		// TODO: get dedenting working (including other fields that support it, like parameter descriptions)
		// {
		// 	// Tests the case where a (dedentable) string value is set that can be
		// 	// pretty-printed as a multi-line string.
		// 	name: "dedent",
		// 	slug: "my_task",
		// 	def: definitions.Definition{
		// 		Slug:        "my_task",
		// 		Description: "An updated description that spans a few lines:\n\n- Attempt 1\n- Attempt 2",
		// 	},
		// },
		// TODO: get comments working
		// {
		// 	// Tests the case where values have comments which should be copied over.
		// 	name: "comments",
		// 	slug: "my_task",
		// 	def: definitions.Definition{
		// 		Slug: "my_task",
		// 		Name: "This task is mine",
		// 	},
		// },

		// TODO: add tests that cover TypeScript
		// TODO: support `import { task } from 'airplane'` syntax where it won't be a member expression (and ignore other functions called task)

		// Test various error conditions:
		// TODO: test airplane.task call without params or with an invalid first argument
		// TODO: audit all possible errors + confirm return user-friendly errors
		// TODO: audit unexpected config format error
		// TODO: check error when there is no matching airplane.task call
		// TODO: test case where we can't update the task for some reason (e.g. parsing), and that we get a sensible error back
	}
	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			require := require.New(t)

			// Clone the input file into a temporary directory as it will be overwritten by `Update()`.
			in, err := os.Open(fmt.Sprintf("./javascript/fixtures/%s.airplane.js", tC.name))
			require.NoError(err)
			f, err := os.CreateTemp("", "runtime-update-javascript-*.airplane.js")
			require.NoError(err)
			t.Cleanup(func() {
				require.NoError(os.Remove(f.Name()))
			})
			_, err = io.Copy(f, in)
			require.NoError(err)
			require.NoError(f.Close())

			l := &logger.MockLogger{}

			ok, err := CanUpdateJavaScriptTask(context.Background(), l, f.Name(), tC.slug)
			require.NoError(err)
			require.True(ok)

			// Perform the update on the temporary file.
			err = UpdateJavaScriptTask(context.Background(), l, f.Name(), tC.slug, tC.def)
			require.NoError(err)

			// Compare
			actual, err := os.ReadFile(f.Name())
			require.NoError(err)
			expected, err := os.ReadFile(fmt.Sprintf("./javascript/fixtures/%s.out.airplane.js", tC.name))
			require.NoError(err)
			require.Equal(string(expected), string(actual))
		})
	}
}

func TestCanUpdateJavaScriptTask(t *testing.T) {
	testCases := []struct {
		slug      string
		canUpdate bool
	}{
		{
			slug:      "spread",
			canUpdate: false,
		},
		{
			slug:      "computed",
			canUpdate: false,
		},
		{
			slug:      "key",
			canUpdate: false,
		},
		{
			slug:      "template",
			canUpdate: false,
		},
		{
			slug:      "tagged_template",
			canUpdate: false,
		},
		{
			// There is no task that matches this slug.
			slug:      "slug_not_found",
			canUpdate: false,
		},
		{
			slug:      "good",
			canUpdate: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.slug, func(t *testing.T) {
			require := require.New(t)

			l := &logger.MockLogger{}

			canUpdate, err := CanUpdateJavaScriptTask(context.Background(), l, "./javascript/fixtures/can_update.airplane.js", tC.slug)
			require.NoError(err)
			require.Equal(tC.canUpdate, canUpdate)
		})
	}
}

func TestFixtures(t *testing.T) {
	require := require.New(t)

	// Assert that the "tabs" fixtures contains tab indentation. This guards against
	// an IDE reverting the indentation in that file.
	for _, file := range []string{"./javascript/fixtures/tabs.airplane.js", "./javascript/fixtures/tabs.out.airplane.js"} {
		contents, err := os.ReadFile(file)
		require.NoError(err)
		require.Contains(string(contents), "\t")
	}
}
