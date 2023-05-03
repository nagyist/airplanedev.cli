package javascript

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/build/types"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/examples"
	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/runtime/runtimetest"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/stretchr/testify/require"
)

func TestFormatComment(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	r := Runtime{}

	require.Equal("// test", r.FormatComment("test"))
	require.Equal(`// line 1
// line 2`, r.FormatComment(`line 1
line 2`))
}

func TestDev(tt *testing.T) {
	tt.Parallel()
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
	t.Parallel()
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
		tC := tC // rebind for parallel tests
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()
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
func TestGenerateInline(t *testing.T) {
	testCases := []struct {
		desc     string
		def      definitions.Definition
		expected string
	}{
		{
			desc: "simple",
			def: definitions.Definition{
				Slug: "my_task",
				Name: "My Task",
				Node: &definitions.NodeDefinition{},
			},
			expected: "simple.ts",
		},
		{
			desc: "default run permissions task participants",
			def: definitions.Definition{
				Slug:                  "my_task",
				Name:                  "My Task",
				DefaultRunPermissions: definitions.NewDefaultTaskViewersDefinition(api.DefaultRunPermissionTaskParticipants),
				Node:                  &definitions.NodeDefinition{},
			},
			expected: "default_run_permissions.ts",
		},
		{
			desc: "default run permissions task-viewer",
			def: definitions.Definition{
				Slug:                  "my_task",
				Name:                  "My Task",
				DefaultRunPermissions: definitions.NewDefaultTaskViewersDefinition(api.DefaultRunPermissionTaskViewers),
				Node:                  &definitions.NodeDefinition{},
			},
			expected: "simple.ts",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)

			r, err := runtime.Lookup(".js", types.TaskKindNode)
			require.NoError(err)

			bytes, _, err := r.GenerateInline(&tC.def)
			require.NoError(err)

			fixtureString, err := os.ReadFile(fmt.Sprintf("./fixtures/generate/%s", tC.expected))
			require.NoError(err)

			require.Equal(string(fixtureString), string(bytes))
		})
	}
}
