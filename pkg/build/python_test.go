package build

import (
	"context"
	"testing"
)

func TestPythonBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []Test{
		{
			Root: "python/simple",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
		},
		{
			Root: "python/simple",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
				"base":       BuildBaseSlim,
			},
		},
		{
			Root: "python/requirements",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			SearchString: "[1]",
		},
		{
			Root: "python/embeddedrequirements",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			SearchString: "[1]",
		},
		{
			Root: "python/requirementswithbuildargs",
			Kind: TaskKindPython,
			Options: KindOptions{
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
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			SearchString: "preinstall='hello from preinstall' postinstall='hello from postinstall'",
		},
		{
			Root: "python/installhooksviashellsubdirectory",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "foo/bar/main.py",
			},
			SearchString: "preinstall='hello from preinstall' postinstall='hello from postinstall'",
		},
		{
			Root: "python/installhooksviaairplaneconfig",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			SearchString: "preinstall='hello from preinstall' postinstall='hello from postinstall'",
		},
	}

	RunTests(t, ctx, tests)
}

func TestPythonBundleBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []Test{
		{
			Root: "python/simple",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim": "true",
			},
			Bundle: true,
			ParamValues: map[string]interface{}{
				"hello": "world",
			},
			BuildContext: BuildContext{
				Type:    PythonBuildType,
				Version: BuildTypeVersionPython310,
			},
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.py",
					SearchString:  "'hello': 'world'",
				},
			},
		},
		{
			Root: "python/simple",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim": "true",
			},
			Bundle: true,
			ParamValues: map[string]interface{}{
				"hello": "world",
			},
			BuildContext: BuildContext{
				Type:    PythonBuildType,
				Version: BuildTypeVersionPython310,
				Base:    BuildBaseSlim,
			},
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.py",
					SearchString:  "'hello': 'world'",
				},
			},
		},
		{
			Root: "python/requirements",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: BuildContext{
				Type:    PythonBuildType,
				Version: BuildTypeVersionPython310,
				Base:    BuildBaseSlim,
			},
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.py",
					SearchString:  "[1]",
				},
			},
		},
		{
			Root: "python/embeddedrequirements",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: BuildContext{
				Type:    PythonBuildType,
				Version: BuildTypeVersionPython310,
				Base:    BuildBaseSlim,
			},
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.py",
					SearchString:  "[1]",
				},
			},
		},
		{
			Root: "python/installhooksviashell",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: BuildContext{
				Type:    PythonBuildType,
				Version: BuildTypeVersionPython310,
				Base:    BuildBaseSlim,
			},
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.py",
					SearchString:  "preinstall='hello from preinstall' postinstall='hello from postinstall'",
				},
			},
		},
		{
			Root: "python/installhooksviashellsubdirectory",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: BuildContext{
				Type:    PythonBuildType,
				Version: BuildTypeVersionPython310,
				Base:    BuildBaseSlim,
			},
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "foo/bar/main.py",
					SearchString:  "preinstall='hello from preinstall' postinstall='hello from postinstall'",
				},
			},
		},
		{
			Root: "python/installhooksviaairplaneconfig",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.py",
			},
			SearchString: "preinstall='hello from preinstall' postinstall='hello from postinstall'",
		},
		{
			Root: "python/bundle",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim": "true",
			},
			Bundle: true,
			ParamValues: map[string]interface{}{
				"name": "pikachu",
			},
			BuildContext: BuildContext{
				Type:    PythonBuildType,
				Version: BuildTypeVersionPython310,
			},
			FilesToDiscover: []string{
				"rootInlineTask_airplane.py",
				"subfolder/subfolderInlineTask_airplane.py",
			},
			BundleRuns: []BundleTestRun{
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
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: BuildContext{
				Type:    PythonBuildType,
				Version: BuildTypeVersionPython310,
			},
			FilesToDiscover: []string{
				"taskmod/task/my_task_airplane.py",
			},
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "taskmod/task/my_task_airplane.py",
					ExportName:    "import_task",
					SearchString:  "running:import_task",
				},
			},
		},
		{
			Root: "python/moduleerror",
			Kind: TaskKindPython,
			Options: KindOptions{
				"shim": "true",
			},
			Bundle: true,
			BuildContext: BuildContext{
				Type:    PythonBuildType,
				Version: BuildTypeVersionPython310,
			},
			FilesToDiscover: []string{
				"main_airplane.py",
			},
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main_airplane.py",
					ExportName:    "my_task",
					SearchString:  `airplane_output_set:["error"] "Test"`,
				},
			},
			ExpectedStatusCode: 1,
		},
	}

	RunTests(t, ctx, tests)
}
