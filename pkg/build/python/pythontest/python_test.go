package pythontest

import (
	"context"
	"testing"

	"github.com/airplanedev/cli/pkg/build"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
)

func TestPythonBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []build.Test{
		{
			Root: "python/simple",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
		},
		{
			Root: "python/simple",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
				"base":       buildtypes.BuildBaseSlim,
			},
		},
		{
			Root: "python/requirements",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			SearchString: "[1]",
		},
		{
			Root: "python/embeddedrequirements",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			SearchString: "[1]",
		},
		{
			Root: "python/requirementswithbuildargs",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			BuildArgs: map[string]string{
				"VER": "3.1.0",
			},
			SearchString: "[1]",
		},
		{
			Root: "python/installhooksviashell",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			SearchString: "preinstall='hello from preinstall' postinstall='hello from postinstall'",
		},
		{
			Root: "python/installhooksviashellsubdirectory",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "foo/bar/main.py",
			},
			SearchString: "preinstall='hello from preinstall' postinstall='hello from postinstall'",
		},
		{
			Root: "python/installhooksviaairplaneconfig",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			SearchString: "preinstall='hello from preinstall' postinstall='hello from postinstall'",
		},
	}

	build.RunTests(t, ctx, tests)
}

func TestPythonBundleBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []build.Test{
		{
			Root: "python/simple",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			ParamValues: map[string]interface{}{
				"hello": "world",
			},
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.PythonBuildType,
				Version: buildtypes.BuildTypeVersionPython310,
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.py",
					SearchString:  "'hello': 'world'",
				},
			},
		},
		{
			Root: "python/simple",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			ParamValues: map[string]interface{}{
				"hello": "world",
			},
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.PythonBuildType,
				Version: buildtypes.BuildTypeVersionPython310,
				Base:    buildtypes.BuildBaseSlim,
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.py",
					SearchString:  "'hello': 'world'",
				},
			},
		},
		{
			Root: "python/requirements",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.PythonBuildType,
				Version: buildtypes.BuildTypeVersionPython310,
				Base:    buildtypes.BuildBaseSlim,
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.py",
					SearchString:  "[1]",
				},
			},
		},
		{
			Root: "python/embeddedrequirements",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.PythonBuildType,
				Version: buildtypes.BuildTypeVersionPython310,
				Base:    buildtypes.BuildBaseSlim,
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.py",
					SearchString:  "[1]",
				},
			},
		},
		{
			Root: "python/installhooksviashell",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.PythonBuildType,
				Version: buildtypes.BuildTypeVersionPython310,
				Base:    buildtypes.BuildBaseSlim,
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.py",
					SearchString:  "preinstall='hello from preinstall' postinstall='hello from postinstall'",
				},
			},
		},
		{
			Root: "python/installhooksviashellsubdirectory",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.PythonBuildType,
				Version: buildtypes.BuildTypeVersionPython310,
				Base:    buildtypes.BuildBaseSlim,
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "foo/bar/main.py",
					SearchString:  "preinstall='hello from preinstall' postinstall='hello from postinstall'",
				},
			},
		},
		{
			Root: "python/installhooksviaairplaneconfig",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			SearchString: "preinstall='hello from preinstall' postinstall='hello from postinstall'",
		},
		{
			Root: "python/bundle",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			ParamValues: map[string]interface{}{
				"name": "pikachu",
			},
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.PythonBuildType,
				Version: buildtypes.BuildTypeVersionPython310,
			},
			FilesToDiscover: []string{
				"rootInlineTask_airplane.py",
				"subfolder/subfolderInlineTask_airplane.py",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "rootInlineTask_airplane.py",
					ExportName:    "my_task",
					SearchString:  "running:my_task",
				},
				{
					RelEntrypoint: "rootInlineTask_airplane.py",
					ExportName:    "my_task2",
					SearchString:  "running:my_task2",
				},
				{
					RelEntrypoint: "subfolder/subfolderInlineTask_airplane.py",
					ExportName:    "my_task3",
					SearchString:  "running:my_task3",
				},
				{
					RelEntrypoint: "subfolder/noninlinetask.py",
					SearchString:  "running:noninlinetask",
				},
			},
		},
		{
			Root: "python/bundleimport",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.PythonBuildType,
				Version: buildtypes.BuildTypeVersionPython310,
			},
			FilesToDiscover: []string{
				"taskmod/task/my_task_airplane.py",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "taskmod/task/my_task_airplane.py",
					ExportName:    "import_task",
					SearchString:  "running:import_task",
				},
			},
		},
		{
			Root: "python/moduleerror",
			Kind: buildtypes.TaskKindPython,
			Options: buildtypes.KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.PythonBuildType,
				Version: buildtypes.BuildTypeVersionPython310,
			},
			FilesToDiscover: []string{
				"main_airplane.py",
			},
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main_airplane.py",
					ExportName:    "my_task",
					SearchString:  `airplane_output_set:["error"] "Test"`,
				},
			},
			ExpectedStatusCode: 1,
		},
	}

	build.RunTests(t, ctx, tests)
}
