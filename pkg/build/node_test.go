package build

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/airplanedev/lib/pkg/examples"
	"github.com/pkg/errors"
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
			Root: "typescript/yarnworkspacespostinstall",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "pkg2/src/index.ts",
				"workdir":    examples.Path(t, "typescript/yarnworkspaces/pkg2"),
			},
			SearchString: "I love airplanes",
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
			Root: "typescript/installhooksviapackagejson",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/installhooksviashell",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/installhooksviashellsubdirectory",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "foo/bar/main.ts",
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/installhooksviapackagejsonoverride",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			SearchString: "hello from preinstall",
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

func TestInlineConfiguredTasks(t *testing.T) {
	ctx := context.Background()

	tests := []Test{
		{
			Root: "typescript/bundle",
			Options: KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: BuildContext{
				Type:    NodeBuildType,
				Version: BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"rootInlineTask.airplane.ts",
				"subfolder/subfolderInlineTask.airplane.ts",
				"subfolder/nonInlineTask.ts",
			},
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "rootInlineTask.airplane.js",
					ExportName:    "default",
					SearchString:  "running:default_export_root_folder",
				},
				{
					RelEntrypoint: "rootInlineTask.airplane.js",
					ExportName:    "named",
					SearchString:  "running:named_export_root_folder",
				},
				{
					RelEntrypoint: "subfolder/subfolderInlineTask.airplane.js",
					ExportName:    "default",
					SearchString:  "running:default_export_subfolder",
				},
				{
					RelEntrypoint: "subfolder/nonInlineTask.js",
					ExportName:    "default",
					SearchString:  "running:non_inline_task",
				},
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
		{
			Root: "javascript/workflowold",
			Kind: TaskKindNode,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.js",
				"runtime":    TaskRuntimeWorkflow,
			},
			ExpectedError: true,
			SkipRun:       true,
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
					"@airplane/workflow-runtime": "0.2.10",
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

			shimPackageJSONSerialized, err := GenShimPackageJSON(
				filepath.Dir(examples.Path(t, tc.packageJSON)),
				packageJSONs,
				tc.isWorkflow,
			)
			require.NoError(err)

			shimJSON := shimPackageJSON{}

			err = json.Unmarshal(shimPackageJSONSerialized, &shimJSON)
			require.NoError(err)

			assert.Equal(tc.expectedShimPackageJSON, shimJSON)
		})
	}

}

func TestReadPackageJSON(t *testing.T) {
	fixturesPath, _ := filepath.Abs("./fixtures")
	testCases := []struct {
		desc                string
		fixture             string
		packageJSON         PackageJSON
		expectNotExistError bool
	}{
		{
			desc:    "reads package.json from file",
			fixture: "node_externals/dependencies/package.json",
			packageJSON: PackageJSON{
				Dependencies:         map[string]string{"react": "18.2.0"},
				DevDependencies:      map[string]string{"@types/react": "18.0.21"},
				OptionalDependencies: map[string]string{"react-table": "7.8.0"},
			},
		},
		{
			desc:    "reads package.json from directory",
			fixture: "node_externals/yarnworkspace",
			packageJSON: PackageJSON{
				DevDependencies: map[string]string{"react": "18.2.0"},
				Workspaces: PackageJSONWorkspaces{
					Workspaces: []string{"lib", "examples/*"},
				},
			},
		},
		{
			desc:                "no package json",
			fixture:             "node_externals",
			expectNotExistError: true,
		},
		{
			desc:                "no package json file",
			fixture:             "node_externals/package.json",
			expectNotExistError: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			path := filepath.Join(fixturesPath, tC.fixture)

			p, err := ReadPackageJSON(path)
			if tC.expectNotExistError {
				assert.True(errors.Is(err, os.ErrNotExist))
				return
			}
			require.NoError(err)

			assert.Equal(tC.packageJSON, p)
		})
	}
}
