package build

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/airplanedev/lib/pkg/examples"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []Test{
		{
			Root: "javascript/simple",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.js",
			},
		},
		{
			Root: "typescript/simple",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/airplaneoverride",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/npm",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/yarn",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/yarn2",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/imports",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "task/main.ts",
			},
		},
		{
			Root: "typescript/noparams",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			// Since this example does not take parameters, override the default SearchString.
			SearchString: "success",
		},
		{
			Root: "typescript/esnext",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/esnext",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":        "true",
				"entrypoint":  "main.ts",
				"nodeVersion": "14",
			},
		},
		{
			Root: "typescript/esm",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/aliases",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/externals",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/yarnworkspaces",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "pkg2/src/index.ts",
				"workdir":    examples.Path(t, "typescript/yarnworkspaces/pkg2"),
			},
		},
		{
			Root: "typescript/yarnworkspacesobject",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "pkg2/src/index.ts",
				"workdir":    examples.Path(t, "typescript/yarnworkspacesobject/pkg2"),
			},
		},
		{
			Root: "typescript/yarnworkspaceswithglob",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "nested/pkg2/src/index.ts",
				"workdir":    examples.Path(t, "typescript/yarnworkspaceswithglob/nested/pkg2"),
			},
		},
		{
			Root: "typescript/npmworkspaces",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "pkg2/src/index.ts",
				"workdir":    examples.Path(t, "typescript/npmworkspaces/pkg2"),
			},
		},
		{
			Root: "typescript/nopackagejson",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/custominstall",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			BuildArgs: map[string]string{
				"IS_PRODUCTION": "false",
			},
		},
		{
			Root: "typescript/custompostinstall",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/prisma",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
	}

	RunTests(t, ctx, tests)
}

func TestNodeWorkflowBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []Test{
		{
			Root: "javascript/workflow",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.js",
				"runtime":    TaskRuntimeWorkflow,
			},
			SkipRun: true,
		},
	}

	RunTests(t, ctx, tests)
}

func TestGenShimPackageJSON(t *testing.T) {
	testCases := []struct {
		desc                    string
		packageJSON             string
		isWorkflow              bool
		expectedShimPackageJSON shimPackageJSON
	}{
		{
			desc:        "Yarn workspace with no shim dependencies",
			packageJSON: "typescript/yarnworkspacesnoairplane/package.json",
			isWorkflow:  true,
			expectedShimPackageJSON: shimPackageJSON{
				Dependencies: map[string]string{
					"airplane":                   defaultSDKVersion,
					"@airplane/workflow-runtime": defaultSDKVersion,
				},
			},
		},
		{
			desc:        "Yarn workspace with shim dependency already included",
			packageJSON: "typescript/yarnworkspaces/package.json",
			isWorkflow:  true,
			expectedShimPackageJSON: shimPackageJSON{
				Dependencies: map[string]string{
					"@airplane/workflow-runtime": "0.2.0-32",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			packageJSONs, _, err := GetPackageJSONs(examples.Path(t, tc.packageJSON))
			require.NoError(err)

			shimPackageJSONSerialized, err := GenShimPackageJSON(packageJSONs, tc.isWorkflow)
			require.NoError(err)

			shimJSON := shimPackageJSON{}

			err = json.Unmarshal(shimPackageJSONSerialized, &shimJSON)
			require.NoError(err)

			assert.Equal(tc.expectedShimPackageJSON, shimJSON)
		})
	}

}
