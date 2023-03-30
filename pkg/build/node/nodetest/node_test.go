package nodetest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/build/node"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/examples"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []build.Test{
		{
			Root: "javascript/simple",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.js",
			},
		},
		{
			Root: "typescript/simple",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/slim",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
				"base":       buildtypes.BuildBaseSlim,
			},
		},
		{
			Root: "typescript/airplaneoverride",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/npm",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/yarn",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/imports",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "task/main.ts",
			},
		},
		{
			Root: "typescript/noparams",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			// Since this example does not take parameters, override the default SearchString.
			SearchString: "success",
		},
		{
			Root: "typescript/esnext",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/esnext",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":        "true",
				"entrypoint":  "main.ts",
				"nodeVersion": "14",
			},
		},
		{
			Root: "typescript/esm",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/aliases",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/externals",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/yarnworkspaces",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "pkg2/src/index.ts",
				"workdir":    examples.Path(t, "typescript/yarnworkspaces/pkg2"),
			},
		},
		{
			Root: "typescript/yarnworkspacesobject",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "pkg2/src/index.ts",
				"workdir":    examples.Path(t, "typescript/yarnworkspacesobject/pkg2"),
			},
		},
		{
			Root: "typescript/yarnworkspaceswithglob",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "nested/pkg2/src/index.ts",
				"workdir":    examples.Path(t, "typescript/yarnworkspaceswithglob/nested/pkg2"),
			},
		},
		{
			Root: "typescript/yarnworkspacespostinstall",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "pkg2/src/index.ts",
				"workdir":    examples.Path(t, "typescript/yarnworkspaces/pkg2"),
			},
			SearchString: "I love airplanes",
		},
		{
			Root: "typescript/npmworkspaces",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "pkg2/src/index.ts",
				"workdir":    examples.Path(t, "typescript/npmworkspaces/pkg2"),
			},
		},
		{
			Root: "typescript/nopackagejson",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
		{
			Root: "typescript/custominstall",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			BuildArgs: map[string]string{
				"IS_PRODUCTION": "false",
			},
		},
		{
			Root: "typescript/installhooksviapackagejson",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/installhooksviaairplaneconfig",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/installhooksviashell",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			SearchString: "rolldice, v1.16",
		},
		{
			Root: "typescript/installhooksviashellsubdirectory",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "foo/bar/main.ts",
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/installhooksviapackagejsonoverride",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/prisma",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
		},
	}

	build.RunTests(t, ctx, tests)
}

func TestNodeBundleBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []build.Test{
		{
			Root: "javascript/simple",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.js",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/simple",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/slim",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
				Base:    buildtypes.BuildBaseSlim,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/airplaneoverride",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/npm",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/yarn",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/yarn2",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
					SearchString:  "3.4.1",
				},
			},
		},
		{
			Root: "typescript/imports",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"task/main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "task/main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/noparams",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
					SearchString:  "success",
				},
			},
			// Since this example does not take parameters, override the default SearchString.
		},
		{
			Root: "typescript/esnext",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/esnext",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode14,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/esm",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/aliases",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/externals",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/yarnworkspaces",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":    "true",
				"workdir": examples.Path(t, "typescript/yarnworkspaces/pkg2"),
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"pkg2/src/index.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "pkg2/src/index.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/yarnworkspacesobject",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":    "true",
				"workdir": examples.Path(t, "typescript/yarnworkspacesobject/pkg2"),
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"pkg2/src/index.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "pkg2/src/index.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/yarnworkspaceswithglob",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":    "true",
				"workdir": examples.Path(t, "typescript/yarnworkspaceswithglob/nested/pkg2"),
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"nested/pkg2/src/index.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "nested/pkg2/src/index.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/yarnworkspacespostinstall",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":    "true",
				"workdir": examples.Path(t, "typescript/yarnworkspaces/pkg2"),
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"pkg2/src/index.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "pkg2/src/index.js",
					ExportName:    "default",
					SearchString:  "I love airplanes",
				},
			},
		},
		{
			Root: "typescript/npmworkspaces",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":    "true",
				"workdir": examples.Path(t, "typescript/npmworkspaces/pkg2"),
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"pkg2/src/index.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "pkg2/src/index.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/nopackagejson",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.ts",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/custominstall",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			BuildArgs: map[string]string{
				"IS_PRODUCTION": "false",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/installhooksviapackagejson",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/installhooksviaairplaneconfig",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/installhooksviashell",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
			SearchString: "rolldice, v1.16",
		},
		{
			Root: "typescript/installhooksviashellsubdirectory",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"foo/bar/main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "foo/bar/main.js",
					ExportName:    "default",
				},
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/installhooksviapackagejsonoverride",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
			SearchString: "hello from preinstall",
		},
		{
			Root: "typescript/prisma",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.js",
					ExportName:    "default",
				},
			},
		},
		{
			Root: "typescript/decorator",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.airplane.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.airplane.js",
					ExportName:    "default",
					SearchString:  "Decorated",
				},
			},
		},
		{
			Root: "typescript/emitDecoratorMetadata",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"main.airplane.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.airplane.js",
					ExportName:    "default",
					SearchString:  "attr1 type: String",
				},
			},
		},
		{
			Root: "typescript/bundle",
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToBuild: []string{
				"rootInlineTask.airplane.ts",
				"taskInView.airplane.tsx",
				"subfolder/subfolderInlineTask.airplane.ts",
				"subfolder/nonInlineTask.ts",
			},
			FilesToDiscover: []string{
				"rootInlineTask.airplane.ts",
				"taskInView.airplane.tsx",
				"subfolder/subfolderInlineTask.airplane.ts",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "rootInlineTask.airplane.js",
					ExportName:    "default",
					SearchString:  "running:default_export_root_folder",
				},
				{
					RelEntrypoint: "taskInView.airplane.js",
					ExportName:    "default",
					SearchString:  "running:in_view",
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
					SearchString:  "running:non_inline_task",
				},
			},
		},
		{
			Root: "typescript/workflowbundle",
			Options: buildtypes.KindOptions{
				"shim":    "true",
				"runtime": buildtypes.TaskRuntimeWorkflow,
			},
			Bundle: true,
			Target: "workflow-build",
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.NodeBuildType,
				Version: buildtypes.BuildTypeVersionNode18,
			},
			FilesToDiscover: []string{
				"workflow.airplane.ts",
				"nested/nested.airplane.ts",
			},
			FilesToBuild: []string{
				"workflow.airplane.ts",
				"nested/nested.airplane.ts",
			},
		},
	}

	build.RunTests(t, ctx, tests)
}

func TestNodeWorkflowBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []build.Test{
		{
			Root: "javascript/workflow",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.js",
				"runtime":    buildtypes.TaskRuntimeWorkflow,
			},
			SkipRun: true,
		},
		{
			Root: "javascript/workflowslim",
			Kind: buildtypes.TaskKindNode,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.js",
				"runtime":    buildtypes.TaskRuntimeWorkflow,
				"base":       buildtypes.BuildBaseSlim,
			},
			SkipRun: true,
		},
		// Test is failing in CI. We should fix this.
		// {
		// 	Root: "javascript/workflowbadimport",
		// 	Kind:  buildtypes.TaskKindNode,
		// 	Options:  buildtypes.KindOptions{
		// 		"shim":       "true",
		// 		"entrypoint": "main.js",
		// 		"runtime":    TaskRuntimeWorkflow,
		// 	},
		// 	ExpectedError: true,
		// 	SkipRun:       true,
		// },
	}

	build.RunTests(t, ctx, tests)
}

func TestGenShimPackageJSON(t *testing.T) {
	var buildToolsPackageJSON node.PackageJSON
	err := json.Unmarshal([]byte(node.BuildToolsPackageJSON), &buildToolsPackageJSON)
	require.NoError(t, err)

	testCases := []struct {
		desc                    string
		packageJSON             string
		isWorkflow              bool
		isBundle                bool
		expectedShimPackageJSON node.ShimPackageJSON
	}{
		{
			desc:        "Yarn workspace with no shim dependencies",
			packageJSON: "typescript/yarnworkspacesnoairplane/package.json",
			isWorkflow:  true,
			expectedShimPackageJSON: node.ShimPackageJSON{
				Dependencies: map[string]string{
					"airplane":                   buildToolsPackageJSON.Dependencies["airplane"],
					"@airplane/workflow-runtime": buildToolsPackageJSON.Dependencies["@airplane/workflow-runtime"],
				},
			},
		},
		{
			desc:        "Yarn workspace with no bundle shim dependencies",
			packageJSON: "typescript/yarnworkspacesnoairplane/package.json",
			isWorkflow:  true,
			isBundle:    true,
			expectedShimPackageJSON: node.ShimPackageJSON{
				Dependencies: map[string]string{
					"airplane":                   buildToolsPackageJSON.Dependencies["airplane"],
					"@airplane/workflow-runtime": buildToolsPackageJSON.Dependencies["@airplane/workflow-runtime"],
					"esbuild":                    buildToolsPackageJSON.Dependencies["esbuild"],
					"jsdom":                      buildToolsPackageJSON.Dependencies["jsdom"],
					"typescript":                 buildToolsPackageJSON.Dependencies["typescript"],
					"esbuild-plugin-tsc":         buildToolsPackageJSON.Dependencies["esbuild-plugin-tsc"],
				},
			},
		},
		{
			desc:        "Yarn workspace with shim dependencies bundle",
			packageJSON: "typescript/yarnworkspacesbundleshimdeps/package.json",
			isWorkflow:  true,
			isBundle:    true,
			expectedShimPackageJSON: node.ShimPackageJSON{
				Dependencies: map[string]string{
					"airplane":                   buildToolsPackageJSON.Dependencies["airplane"],
					"@airplane/workflow-runtime": buildToolsPackageJSON.Dependencies["@airplane/workflow-runtime"],
					"esbuild":                    buildToolsPackageJSON.Dependencies["esbuild"],
					"jsdom":                      buildToolsPackageJSON.Dependencies["jsdom"],
					"typescript":                 "4.9.5",
					"esbuild-plugin-tsc":         buildToolsPackageJSON.Dependencies["esbuild-plugin-tsc"],
				},
			},
		},
		{
			desc:        "Yarn workspace with shim dependency already included",
			packageJSON: "typescript/yarnworkspaces/package.json",
			isWorkflow:  true,
			isBundle:    true,
			expectedShimPackageJSON: node.ShimPackageJSON{
				Dependencies: map[string]string{
					"@airplane/workflow-runtime": buildToolsPackageJSON.Dependencies["@airplane/workflow-runtime"],
					"esbuild":                    buildToolsPackageJSON.Dependencies["esbuild"],
					"jsdom":                      buildToolsPackageJSON.Dependencies["jsdom"],
					"typescript":                 "4.9.5",
					"esbuild-plugin-tsc":         buildToolsPackageJSON.Dependencies["esbuild-plugin-tsc"],
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			packageJSONs, _, err := node.GetPackageJSONs(examples.Path(t, tc.packageJSON))
			require.NoError(err)

			shimPackageJSONSerialized, err := node.GenShimPackageJSON(node.GenShimPackageJSONOpts{
				RootDir:      filepath.Dir(examples.Path(t, tc.packageJSON)),
				PackageJSONs: packageJSONs,
				IsWorkflow:   tc.isWorkflow,
				IsBundle:     tc.isBundle,
			})
			require.NoError(err)

			shimJSON := node.ShimPackageJSON{}

			err = json.Unmarshal(shimPackageJSONSerialized, &shimJSON)
			require.NoError(err)

			assert.Equal(tc.expectedShimPackageJSON, shimJSON)
		})
	}

}

func TestReadPackageJSON(t *testing.T) {
	fixturesPath, _ := filepath.Abs("../../fixtures")
	testCases := []struct {
		desc                string
		fixture             string
		packageJSON         node.PackageJSON
		expectNotExistError bool
	}{
		{
			desc:    "reads package.json from file",
			fixture: "node_externals/dependencies/package.json",
			packageJSON: node.PackageJSON{
				Dependencies:         map[string]string{"react": "18.2.0"},
				DevDependencies:      map[string]string{"@types/react": "18.0.28"},
				OptionalDependencies: map[string]string{"react-table": "7.8.0"},
			},
		},
		{
			desc:    "reads package.json from directory",
			fixture: "node_externals/yarnworkspace",
			packageJSON: node.PackageJSON{
				Name:            "airplane",
				DevDependencies: map[string]string{"react": "18.2.0"},
				Workspaces: node.PackageJSONWorkspaces{
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

			p, err := node.ReadPackageJSON(path)
			if tC.expectNotExistError {
				assert.True(errors.Is(err, os.ErrNotExist))
				return
			}
			require.NoError(err)

			assert.Equal(tC.packageJSON, p)
		})
	}
}
