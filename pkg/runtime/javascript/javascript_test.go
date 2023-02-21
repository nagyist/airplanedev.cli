package javascript

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/examples"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/runtime/runtimetest"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/airplanedev/lib/pkg/utils/pointers"
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
			Kind: build.TaskKindNode,
			Opts: runtime.PrepareRunOptions{
				Path:     "javascript/simple/main.js",
				TaskSlug: "simple",
			},
		},
		{
			Kind: build.TaskKindNode,
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
		buildVersion build.BuildTypeVersion
	}{
		{
			desc:         "single node version",
			path:         "./fixtures/version/18/file.js",
			buildVersion: build.BuildTypeVersionNode18,
		},
		{
			desc:         "greater than node version",
			path:         "./fixtures/version/gt15/file.js",
			buildVersion: build.BuildTypeVersionNode18,
		},
		{
			desc:         "greater than and less than node version",
			path:         "./fixtures/version/gt15lt18/file.js",
			buildVersion: build.BuildTypeVersionNode16,
		},
		{
			desc:         "version from config file",
			path:         "./fixtures/version/fromConfig/file.js",
			buildVersion: build.BuildTypeVersionNode14,
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

			r, err := runtime.Lookup(tC.path, build.TaskKindNode)
			require.NoError(err)

			root, err := r.Root(tC.path)
			require.NoError(err)

			bv, err := r.Version(root)
			require.NoError(err)

			require.Equal(tC.buildVersion, bv)
		})
	}
}

func TestEdit(t *testing.T) {
	testCases := []struct {
		name string
		slug string
		def  definitions.Definition_0_3
	}{
		{
			// Tests setting various fields.
			name: "all",
			slug: "my_task",
			def: definitions.Definition_0_3{
				// This case also tests renaming a task slug.
				Slug:        "my_task_2",
				Name:        "Task name",
				Description: "Task description",
				Parameters: []definitions.ParameterDefinition_0_3{
					{
						Slug:        "dry",
						Name:        "Dry run?",
						Description: "Whether or not to run in dry-run mode.",
						Type:        "boolean",
						Required:    definitions.NewDefaultTrueDefinition(false),
						Default:     true,
					},
				},
				Runtime:            "workflow",
				RequireRequests:    true,
				AllowSelfApprovals: definitions.NewDefaultTrueDefinition(false),
				Timeout:            definitions.NewDefaultTimeoutDefinition(60),
				Constraints: map[string]string{
					"cluster": "k8s",
					"vpc":     "tasks",
				},
				Schedules: map[string]definitions.ScheduleDefinition_0_3{
					"daily": {
						Name:        "Daily",
						CronExpr:    "0 12 * * *",
						Description: "Runs every day at 12 UTC",
						ParamValues: map[string]interface{}{
							"dry": false,
						},
					},
				},
				Resources: definitions.ResourceDefinition_0_3{
					Attachments: map[string]string{
						"db": "db",
					},
				},
				Node: &definitions.NodeDefinition_0_3{
					EnvVars: api.TaskEnv{
						"AWS_ACCESS_KEY": api.EnvVarValue{
							Config: pointers.String("aws_access_key"),
						},
					},
				},
			},
		},
		{
			// Tests the case where values are cleared.
			name: "all_cleared",
			slug: "my_task",
			def: definitions.Definition_0_3{
				Slug: "my_task",
			},
		},
		{
			// Tests the case where values are set to their default values (and therefore should not be serialized).
			name: "all_defaults",
			slug: "my_task",
			def: definitions.Definition_0_3{
				Slug:        "my_task",
				Name:        "",
				Description: "",
				Parameters:  []definitions.ParameterDefinition_0_3{},
				Resources: definitions.ResourceDefinition_0_3{
					Attachments: map[string]string{},
				},
				Node: &definitions.NodeDefinition_0_3{
					EnvVars: api.TaskEnv{},
				},
				Constraints:        map[string]string{},
				RequireRequests:    false,
				AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
				Timeout:            definitions.NewDefaultTimeoutDefinition(3600),
				Runtime:            build.TaskRuntimeStandard,
				Schedules:          map[string]definitions.ScheduleDefinition_0_3{},
			},
		},
		{
			// Tests the case where the slug's key is a string literal ("slug") instead of an identifier (slug).
			name: "slug_string_literal",
			slug: "my_task",
			def: definitions.Definition_0_3{
				Slug: "my_task",
				Name: "This task is mine",
			},
		},
		{
			// Tests the case where the slug's key is a string literal ("slug") instead of an identifier (slug).
			name: "multiple_tasks",
			slug: "my_task_2",
			def: definitions.Definition_0_3{
				Slug: "my_task_two",
				Name: "My task (v2)",
			},
		},
		// TODO: support basic variable references
		// {
		// 	// Tests the case where a task's options are stored in a separate variable.
		// 	name: "variable_opts",
		// 	slug: "my_task",
		// 	def: definitions.Definition_0_3{
		// 		Slug: "my_task",
		// 		Name: "This task is mine",
		// 	},
		// },
		// TODO: get dedenting working
		// TODO: test other dedentable fields, too, like parameter descriptions
		// {
		// 	// Tests the case where a (dedentable) string value is set that can be
		// 	// pretty-printed as a multi-line string.
		// 	name: "dedent",
		// 	slug: "my_task",
		// 	def: definitions.Definition_0_3{
		// 		Slug:        "my_task",
		// 		Description: "An updated description that spans a few lines:\n\n- Attempt 1\n- Attempt 2",
		// 	},
		// },
		// TODO: get comments working
		// {
		// 	// Tests the case where values have comments which should be copied over.
		// 	name: "comments",
		// 	slug: "my_task",
		// 	def: definitions.Definition_0_3{
		// 		Slug: "my_task",
		// 		Name: "This task is mine",
		// 	},
		// },

		// TODO: support `import { task } from 'airplane'` syntax where it won't be a member expression
		// TODO: tolerant parsing
		// TODO: test spaces vs. tabs
		// TODO: add parameter test cases (incl options/regex which aren't supported yet)
		// TODO: add schedule test cases
		// TODO: add resource test cases
		// TODO: non-identifier keys
		// TODO: add tests that cover TypeScript

		// TODO: confirm various error cases return user-friendly errors
		// TODO: ignore non-airplane call expressions even with matching names
		// TODO: detect if fields are explicitly set to a default value + retain the value if so
		// TODO: test case where we can't edit the task for some reason (e.g. parsing), and that we get a sensible error back
		// TODO: support serializing parameters as type-only strings
		// TODO: support views
		// TODO: multiple fields with the same field name
		// TODO: audit all possible errors
		// TODO: audit unexpected config format error
		// TODO: string literal with double/single quotes in it
	}
	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			require := require.New(t)

			r, err := runtime.Lookup(".js", build.TaskKindNode)
			require.NoError(err)

			// Clone the input file into a temporary directory as it will be overwritten by `Edit()`.
			in, err := os.Open(fmt.Sprintf("./fixtures/transformer/%s.airplane.js", tC.name))
			require.NoError(err)
			f, err := os.CreateTemp("", "runtime-edit-javascript-*")
			require.NoError(err)
			t.Cleanup(func() {
				require.NoError(os.Remove(f.Name()))
			})
			_, err = io.Copy(f, in)
			require.NoError(err)
			require.NoError(f.Close())

			l := &logger.MockLogger{}

			// Perform the edit on the temporary file.
			err = r.Edit(context.Background(), l, f.Name(), tC.slug, definitions.DefinitionInterface(&tC.def))
			require.NoError(err)

			// Compare
			actual, err := os.ReadFile(f.Name())
			require.NoError(err)
			expected, err := os.ReadFile(fmt.Sprintf("./fixtures/transformer/%s.out.airplane.js", tC.name))
			require.NoError(err)
			require.Equal(string(expected), string(actual))
		})
	}
}
