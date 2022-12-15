package build

import (
	"context"
	"testing"
)

func TestShellBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []Test{
		{
			Root: "shell/simple",
			Kind: TaskKindShell,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
		},
		{
			Root: "shell/simple",
			Kind: TaskKindShell,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
			ParamValues: map[string]interface{}{
				"param_one": "testtest",
			},
			SearchString: "PARAM_PARAM_ONE=testtest",
		},
		{
			Root: "shell/simple",
			Kind: TaskKindShell,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
			ParamValues: map[string]interface{}{
				"param_one": "firstline\nsecondline",
			},
			SearchString: "PARAM_PARAM_ONE=firstline",
		},
		{
			Root: "shell/ubuntu",
			Kind: TaskKindShell,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
			SearchString: "bar",
		},
		{
			Root: "shell/zcli",
			Kind: TaskKindShell,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
			SearchString: "bar",
		},
		{
			Root: "shell/diff-workdir",
			Kind: TaskKindShell,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
			SearchString: "bar",
		},
		{
			Root: "shell/ubuntu-no-newline",
			Kind: TaskKindShell,
			Options: KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
		},
	}

	RunTests(t, ctx, tests)
}

func TestShellBundleBuilder(t *testing.T) {
	ctx := context.Background()
	tests := []Test{
		{
			Root: "shell/simple",
			Kind: TaskKindShell,
			BuildContext: BuildContext{
				Type: ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "Hello World!",
				},
			},
		},
		{
			Root: "shell/simple",
			Kind: TaskKindShell,
			BuildContext: BuildContext{
				Type: ShellBuildType,
			},
			ParamValues: map[string]interface{}{
				"param_one": "testtest",
			},
			Bundle: true,
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "PARAM_PARAM_ONE=testtest",
				},
			},
		},
		{
			Root: "shell/simple",
			Kind: TaskKindShell,
			BuildContext: BuildContext{
				Type: ShellBuildType,
			},
			ParamValues: map[string]interface{}{
				"param_one": "firstline\nsecondline",
			},
			Bundle: true,
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "PARAM_PARAM_ONE=firstline",
				},
			},
		},
		{
			Root: "shell/ubuntu",
			Kind: TaskKindShell,
			BuildContext: BuildContext{
				Type: ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "bar",
				},
			},
		},
		{
			Root: "shell/zcli",
			Kind: TaskKindShell,
			BuildContext: BuildContext{
				Type: ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "bar",
				},
			},
		},
		{
			Root: "shell/diff-workdir",
			Kind: TaskKindShell,
			BuildContext: BuildContext{
				Type: ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "bar",
				},
			},
		},
		{
			Root: "shell/ubuntu-no-newline",
			Kind: TaskKindShell,
			BuildContext: BuildContext{
				Type: ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "Echoing env variable",
				},
			},
		},
		{
			Root: "shell/multiplescripts",
			Kind: TaskKindShell,
			BuildContext: BuildContext{
				Type: ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "one.sh",
					SearchString:  "script 1",
				},
				{
					RelEntrypoint: "two.sh",
					SearchString:  "script 2",
				},
				{
					RelEntrypoint: "nested/three.sh",
					SearchString:  "script 3",
				},
			},
		},
		{
			Root: "shell/dockerfilewithentrypoint",
			Kind: TaskKindShell,
			BuildContext: BuildContext{
				Type: ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "bar",
				},
			},
		},
	}

	RunTests(t, ctx, tests)
}
