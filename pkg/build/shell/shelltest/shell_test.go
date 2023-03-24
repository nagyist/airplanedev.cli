package shelltest

import (
	"context"
	"testing"

	"github.com/airplanedev/lib/pkg/build"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
)

func TestShellBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []build.Test{
		{
			Root: "shell/simple",
			Kind: buildtypes.TaskKindShell,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
		},
		{
			Root: "shell/simple",
			Kind: buildtypes.TaskKindShell,
			Options: buildtypes.KindOptions{
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
			Kind: buildtypes.TaskKindShell,
			Options: buildtypes.KindOptions{
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
			Kind: buildtypes.TaskKindShell,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
			SearchString: "bar",
		},
		{
			Root: "shell/zcli",
			Kind: buildtypes.TaskKindShell,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
			SearchString: "bar",
		},
		{
			Root: "shell/diff-workdir",
			Kind: buildtypes.TaskKindShell,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
			SearchString: "bar",
		},
		{
			Root: "shell/ubuntu-no-newline",
			Kind: buildtypes.TaskKindShell,
			Options: buildtypes.KindOptions{
				"shim":       "true",
				"entrypoint": "main.sh",
			},
		},
	}

	build.RunTests(t, ctx, tests)
}

func TestShellBundleBuilder(t *testing.T) {
	ctx := context.Background()
	tests := []build.Test{
		{
			Root: "shell/simple",
			Kind: buildtypes.TaskKindShell,
			BuildContext: buildtypes.BuildContext{
				Type: buildtypes.ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "Hello World!",
				},
			},
		},
		{
			Root: "shell/simple",
			Kind: buildtypes.TaskKindShell,
			BuildContext: buildtypes.BuildContext{
				Type: buildtypes.ShellBuildType,
			},
			ParamValues: map[string]interface{}{
				"param_one": "testtest",
			},
			Bundle: true,
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "PARAM_PARAM_ONE=testtest",
				},
			},
		},
		{
			Root: "shell/simple",
			Kind: buildtypes.TaskKindShell,
			BuildContext: buildtypes.BuildContext{
				Type: buildtypes.ShellBuildType,
			},
			ParamValues: map[string]interface{}{
				"param_one": "firstline\nsecondline",
			},
			Bundle: true,
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "PARAM_PARAM_ONE=firstline",
				},
			},
		},
		{
			Root: "shell/ubuntu",
			Kind: buildtypes.TaskKindShell,
			BuildContext: buildtypes.BuildContext{
				Type: buildtypes.ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "bar",
				},
			},
		},
		{
			Root: "shell/zcli",
			Kind: buildtypes.TaskKindShell,
			BuildContext: buildtypes.BuildContext{
				Type: buildtypes.ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "bar",
				},
			},
		},
		{
			Root: "shell/diff-workdir",
			Kind: buildtypes.TaskKindShell,
			BuildContext: buildtypes.BuildContext{
				Type: buildtypes.ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "bar",
				},
			},
		},
		{
			Root: "shell/ubuntu-no-newline",
			Kind: buildtypes.TaskKindShell,
			BuildContext: buildtypes.BuildContext{
				Type: buildtypes.ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "Echoing env variable",
				},
			},
		},
		{
			Root: "shell/multiplescripts",
			Kind: buildtypes.TaskKindShell,
			BuildContext: buildtypes.BuildContext{
				Type: buildtypes.ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []build.BundleTestRun{
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
			Kind: buildtypes.TaskKindShell,
			BuildContext: buildtypes.BuildContext{
				Type: buildtypes.ShellBuildType,
			},
			Bundle: true,
			BundleRuns: []build.BundleTestRun{
				{
					RelEntrypoint: "main.sh",
					SearchString:  "bar",
				},
			},
		},
	}

	build.RunTests(t, ctx, tests)
}
