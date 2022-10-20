package javascript

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/examples"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/runtime/runtimetest"
	"github.com/airplanedev/lib/pkg/utils/fsx"
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
			Opts: runtime.PrepareRunOptions{Path: "javascript/simple/main.js"},
		},
		{
			Kind: build.TaskKindNode,
			Opts: runtime.PrepareRunOptions{Path: "javascript/customroot/main.js"},
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
			desc:         "great than node version",
			path:         "./fixtures/version/gt15/file.js",
			buildVersion: build.BuildTypeVersionNode18,
		},
		{
			desc:         "great than and less than node version",
			path:         "./fixtures/version/gt15lt18/file.js",
			buildVersion: build.BuildTypeVersionNode16,
		},
		{
			desc: "no version",
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
