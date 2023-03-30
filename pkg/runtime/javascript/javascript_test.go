package javascript

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/build/types"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/cli/pkg/examples"
	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/runtime/runtimetest"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/stretchr/testify/require"
)

func TestFormatComment(t *testing.T) {
	require := require.New(t)

	r := Runtime{}

	require.Equal("// test", r.FormatComment("test"))
	require.Equal(`// line 1
// line 2`, r.FormatComment(`line 1
line 2`))
}

func TestDev(tt *testing.T) {
	ctx := context.Background()

	tests := []runtimetest.Test{
		{
			Kind: buildtypes.TaskKindNode,
			Opts: runtime.PrepareRunOptions{
				Path:     "javascript/simple/main.js",
				TaskSlug: "simple",
			},
		},
		{
			Kind: buildtypes.TaskKindNode,
			Opts: runtime.PrepareRunOptions{
				Path:     "javascript/customroot/main.js",
				TaskSlug: "custom",
			},
		},
		// This test can fail depending on the order in which packages are loaded
		// since it depends on the typescript runtime being registered.
		//
		// TODO: Move it to the typescript package to avoid this problem.
		//{
		//	Kind: build.TaskKindNode,
		//	Opts: runtime.PrepareRunOptions{Path: "typescript/yarnworkspaces/pkg2/src/index.ts"},
		//},
	}

	// For the dev workflow, we expect users to run `npm install` themselves before
	// running the dev command. Therefore, perform an `npm install` on each example:
	for _, test := range tests {
		p := examples.Path(tt, test.Opts.Path)

		// Check if this example uses npm or yarn:
		r, err := runtime.Lookup(p, test.Kind)
		require.NoError(tt, err)
		root, err := r.Root(p)
		require.NoError(tt, err)
		var cmd *exec.Cmd
		if fsx.Exists(filepath.Join(root, "yarn.lock")) {
			os.Remove(filepath.Join(root, "yarn.lock"))
			cmd = exec.CommandContext(ctx, "yarn")
		} else {
			cmd = exec.CommandContext(ctx, "npm", "install", "--no-save")
		}

		// Install dependencies:
		workdir, err := r.Workdir(p)
		require.NoError(tt, err)
		cmd.Dir = workdir
		out, err := cmd.CombinedOutput()
		require.NoError(tt, err, "Failed to run %q for %q:\n%s", cmd.String(), test.Opts.Path, string(out))
	}

	runtimetest.Run(tt, ctx, tests)
}

func TestVersion(t *testing.T) {
	testCases := []struct {
		desc         string
		path         string
		buildVersion buildtypes.BuildTypeVersion
	}{
		{
			desc:         "single node version",
			path:         "./fixtures/version/18/file.js",
			buildVersion: buildtypes.BuildTypeVersionNode18,
		},
		{
			desc:         "greater than node version",
			path:         "./fixtures/version/gt15/file.js",
			buildVersion: buildtypes.BuildTypeVersionNode18,
		},
		{
			desc:         "greater than and less than node version",
			path:         "./fixtures/version/gt15lt18/file.js",
			buildVersion: buildtypes.BuildTypeVersionNode16,
		},
		{
			desc:         "version from config file",
			path:         "./fixtures/version/fromConfig/file.js",
			buildVersion: buildtypes.BuildTypeVersionNode14,
		},
		{
			desc: "no version",
			path: "./fixtures/version/emptyPackageJSON/file.js",
		},
		{
			desc: "no package.json",
			path: "./fixtures/version/empty/file.js",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)

			r, err := runtime.Lookup(tC.path, buildtypes.TaskKindNode)
			require.NoError(err)

			root, err := r.Root(tC.path)
			require.NoError(err)

			bv, err := r.Version(root)
			require.NoError(err)

			require.Equal(tC.buildVersion, bv)
		})
	}
}

func TestUpdate(t *testing.T) {
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

			r, err := runtime.Lookup(".js", buildtypes.TaskKindNode)
			require.NoError(err)

			// Clone the input file into a temporary directory as it will be overwritten by `Update()`.
			in, err := os.Open(fmt.Sprintf("./fixtures/update/%s.airplane.js", tC.name))
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

			ok, err := r.CanUpdate(context.Background(), l, f.Name(), tC.slug)
			require.NoError(err)
			require.True(ok)

			// Perform the update on the temporary file.
			err = r.Update(context.Background(), l, f.Name(), tC.slug, tC.def)
			require.NoError(err)

			// Compare
			actual, err := os.ReadFile(f.Name())
			require.NoError(err)
			expected, err := os.ReadFile(fmt.Sprintf("./fixtures/update/%s.out.airplane.js", tC.name))
			require.NoError(err)
			require.Equal(string(expected), string(actual))
		})
	}
}

func TestCanUpdate(t *testing.T) {
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

			r, err := runtime.Lookup(".js", types.TaskKindNode)
			require.NoError(err)

			l := &logger.MockLogger{}

			canUpdate, err := r.CanUpdate(context.Background(), l, "./fixtures/update/can_update.airplane.js", tC.slug)
			require.NoError(err)
			require.Equal(tC.canUpdate, canUpdate)
		})
	}
}

func TestFixtures(t *testing.T) {
	require := require.New(t)

	// Assert that the "tabs" fixtures contains tab indentation. This guards against
	// an IDE reverting the indentation in that file.
	for _, file := range []string{"./fixtures/update/tabs.airplane.js", "./fixtures/update/tabs.out.airplane.js"} {
		contents, err := os.ReadFile(file)
		require.NoError(err)
		require.Contains(string(contents), "\t")
	}
}
