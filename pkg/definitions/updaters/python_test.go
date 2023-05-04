package updaters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/deploy/config"
	"github.com/airplanedev/cli/pkg/testutils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/stretchr/testify/require"
)

func TestUpdatePythonTask(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		slug      string
		def       definitions.Definition
		pyVersion string
		err       string
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
				Python: &definitions.PythonDefinition{
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
			// Tests the case where values are cleared.
			// Note that the imports are not removed since we can't check if they were used elsewhere.
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
				Python: &definitions.PythonDefinition{
					EnvVars: api.EnvVars{},
				},
				Constraints:        map[string]string{},
				RequireRequests:    false,
				AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
				RestrictCallers:    []string{},
				Timeout:            3600,
				Runtime:            buildtypes.TaskRuntimeStandard,
				Schedules:          map[string]definitions.ScheduleDefinition{},
			},
		},
		{
			// Tests edge cases for string values:
			// 1. A field with a normal string.
			// 2. A field is updated that includes single quotes.
			// 3. A field is updated that includes double quotes.
			// 4. A field is updated that includes both quotes.
			name: "strings",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Constraints: map[string]string{
					"a_valid_identifier": "...",
					"double\"":           "\"",
					"single'":            "'",
					"both'\"'\"":         "'\"'\"",
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
				Parameters: []definitions.ParameterDefinition{
					{
						Slug:        "dry",
						Name:        "Dry run?",
						Description: "Whether or not to run in dry-run mode.",
						Type:        "boolean",
						Required:    definitions.NewDefaultTrueDefinition(false),
						Default:     true,
					},
				},
				Python: &definitions.PythonDefinition{
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
			// Tests the case where a task uses 2 spaces instead of 4.
			name: "spaces",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Name: "Task name",
				Parameters: []definitions.ParameterDefinition{
					{
						Slug:        "dry",
						Name:        "Dry run?",
						Description: "Whether or not to run in dry-run mode.",
						Type:        "boolean",
						Required:    definitions.NewDefaultTrueDefinition(false),
						Default:     true,
					},
				},
				Python: &definitions.PythonDefinition{
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
						// Test another ISO8601 format string.
						Slug:    "datetime2",
						Type:    "datetime",
						Default: "2006-01-02T15:04:05.123Z",
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
					{
						// This parameter does not have a default value. It should be moved up before all
						// other parameters that have defaults.
						Slug: "default_name",
						Type: "shorttext",
						// This name can be generated from the slug. It should not be serialized.
						Name: "Default name",
					},
					{
						// This parameter does not have a default value, but is optional. It should stay here
						// rather than being moved up like `default_name`.
						Slug:     "default_name_optional",
						Type:     "shorttext",
						Required: definitions.NewDefaultTrueDefinition(false),
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
			// Tests the case where the file contains invalid Python.
			// This case also checks the case where there are no existing import lines.
			// This case also checks that "import airplane" is added, if not present.
			name: "invalid_code",
			slug: "my_task",
			def: definitions.Definition{
				Slug:        "my_task",
				Description: "Added a description!",
			},
		},
		{
			// Tests the case where a file has various kinds of imports.
			name: "imports",
			slug: "my_task",
			def: definitions.Definition{
				Slug:        "my_task",
				Description: "Added a description!",
				Parameters: []definitions.ParameterDefinition{
					{
						// This will require adding a new import.
						Slug: "datetime",
						Type: "datetime",
					},
				},
			},
		},
		{
			// Tests the case where a file has multiple decorators.
			name: "decorators",
			slug: "my_task",
			def: definitions.Definition{
				Slug:        "my_task",
				Description: "Added a description!",
				Parameters: []definitions.ParameterDefinition{
					{
						Slug: "name",
						Type: "shorttext",
					},
				},
			},
		},
		{
			// Tests the case where a task slug is not found.
			name: "not_found",
			slug: "my_task_not_found",
			def: definitions.Definition{
				Slug: "my_task_not_found",
			},
			err: `Could not find task with slug "my_task_not_found"`,
		},
		{
			// Tests the case where a task uses modern Python syntax, including return types
			// and async def.
			name: "modern_syntax",
			slug: "my_task",
			def: definitions.Definition{
				Slug:        "my_task",
				Description: "Added a description!",
				Parameters: []definitions.ParameterDefinition{
					{
						Slug: "name",
						Type: "shorttext",
					},
				},
			},
		},
		{
			// Tests the case where a slug is set explicitly via the decorator and the function
			// name is unrelated. The function name should not be changed.
			name: "explicit_slug",
			slug: "my_task",
			def: definitions.Definition{
				Slug:        "my_task_2",
				Description: "Added a description!",
				Parameters: []definitions.ParameterDefinition{
					{
						Slug: "name",
						Type: "shorttext",
					},
				},
			},
		},
		{
			// Tests the case where a task includes multi-byte characters.
			name: "unicode",
			slug: "my_task",
			def: definitions.Definition{
				Slug:        "my_task",
				Name:        "你好世界, ߷, or 👨‍👩‍👧‍👧",
				Description: "你好世界, ߷, or 👨‍👩‍👧‍👧",
				Parameters: []definitions.ParameterDefinition{
					{
						Slug:        "name",
						Type:        "shorttext",
						Name:        "你好世界, ߷, or 👨‍👩‍👧‍👧",
						Description: "你好世界, ߷, or 👨‍👩‍👧‍👧",
					},
				},
			},
		},
		{
			// Tests the case where code is serialized for Python 3.9.
			name: "py39",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Parameters: []definitions.ParameterDefinition{
					{
						Slug:     "name",
						Type:     "shorttext",
						Name:     "User name",
						Required: definitions.NewDefaultTrueDefinition(false),
					},
				},
			},
			pyVersion: "3.9",
		},
		{
			// Tests the case where code is serialized for Python 3.10.
			name: "py310",
			slug: "my_task",
			def: definitions.Definition{
				Slug: "my_task",
				Parameters: []definitions.ParameterDefinition{
					{
						Slug:     "name",
						Type:     "shorttext",
						Name:     "User name",
						Required: definitions.NewDefaultTrueDefinition(false),
					},
				},
			},
			pyVersion: "3.10",
		},
	}

	// To make these tests portable, tell the updater to ignore the current Python version.
	// This way, it will generate code matching the airplane.yaml version above.
	testingOnlyIgnoreCurrentPythonVersion = true

	for _, tC := range testCases {
		tC := tC // rebind for parallel tests
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()
			require := require.New(t)
			l := logger.NewTestLogger(t)

			// Clone the input file into a temporary directory as it will be overwritten by `Update()`.
			in, err := os.ReadFile(fmt.Sprintf("./python/fixtures/%s_airplane.py", tC.name))
			require.NoError(err)
			dir := testutils.Tempdir(t)
			path := filepath.Join(dir, "in_airplane.py")
			err = os.WriteFile(path, in, 0755)
			require.NoError(err)

			// Serialize an airplane.yaml with the selected Python version to use for these tests.
			if tC.pyVersion == "" {
				// By default, generate for the oldest supported version of Python.
				tC.pyVersion = "3.8"
			}
			c := config.AirplaneConfig{Python: config.PythonConfig{Version: tC.pyVersion}}
			airplaneYAMLBytes, err := json.Marshal(c)
			require.NoError(err)
			err = os.WriteFile(filepath.Join(dir, "airplane.yaml"), airplaneYAMLBytes, 0755)
			require.NoError(err)

			ok, err := CanUpdatePythonTask(context.Background(), l, path, tC.slug)
			if tC.err == "" {
				require.NoError(err)
				require.True(ok)
			} else {
				require.NoError(err)
				require.False(ok)
			}

			// Perform the update on the temporary file.
			err = UpdatePythonTask(context.Background(), l, dir, path, tC.slug, tC.def)
			if tC.err == "" {
				require.NoError(err)
			} else {
				require.True(strings.HasSuffix(err.Error(), ": "+tC.err))
			}

			// Compare
			actual, err := os.ReadFile(path)
			require.NoError(err)
			expected, err := os.ReadFile(fmt.Sprintf("./python/fixtures/%s_out_airplane.py", tC.name))
			require.NoError(err)
			require.Equal(string(expected), string(actual))
		})
	}
}

func TestCanUpdatePythonTask(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		slug      string
		canUpdate bool
	}{
		{
			// Sets a task field to a conditional value.
			slug:      "conditional",
			canUpdate: false,
		},
		{
			// Sets a task field to a shared variable.
			slug:      "shared_value",
			canUpdate: false,
		},
		{
			// Sets a parameter default to a shared variable.
			slug:      "shared_default",
			canUpdate: false,
		},
		{
			// Has a computed slug.
			slug:      "computed_slug",
			canUpdate: false,
		},
		{
			// Uses a template string.
			slug:      "template",
			canUpdate: false,
		},
		{
			// There is no task that matches this slug.
			slug:      "slug_not_found",
			canUpdate: false,
		},
		{
			// There is a class method with this name and an airplane.task decorator.
			// We only support the decorator on functions, so this should not work.
			slug:      "run",
			canUpdate: false,
		},
		{
			// A perfectly fine task, but located in the same file as all the tasks above.
			slug:      "good",
			canUpdate: true,
		},
	}
	for _, tC := range testCases {
		tC := tC // rebind for parallel tests
		t.Run(tC.slug, func(t *testing.T) {
			t.Parallel()
			require := require.New(t)
			l := logger.NewTestLogger(t)

			canUpdate, err := CanUpdatePythonTask(context.Background(), l, "./python/fixtures/can_update_airplane.py", tC.slug)
			require.NoError(err)
			require.Equal(tC.canUpdate, canUpdate)
		})
	}
}

func TestPythonTaskFixtures(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Assert that the "tabs" fixtures contains tab indentation. This guards against
	// an IDE reverting the indentation in that file.
	for _, file := range []string{"./python/fixtures/tabs_airplane.py", "./python/fixtures/tabs_out_airplane.py"} {
		contents, err := os.ReadFile(file)
		require.NoError(err)
		require.Contains(string(contents), "\t")
	}

	// Assert that the "2-spaces" fixtures use 2 spaces instead of 4 spaces. This guards against
	// an IDE reverting the indentation in that file.
	for _, file := range []string{"./python/fixtures/spaces_airplane.py", "./python/fixtures/spaces_out_airplane.py"} {
		contents, err := os.ReadFile(file)
		require.NoError(err)
		// Assert that at least one line has exactly two spaces of indentation.
		lines := strings.Split(string(contents), "\n")
		ok := false
		r, err := regexp.Compile(`^  \b`)
		require.NoError(err)
		for _, line := range lines {
			ok = ok || r.MatchString(line)
		}
		require.True(ok, "Expected file %s to be indented by two spaces", file)
	}
}
