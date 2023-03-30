package viewstest

import (
	"context"
	"testing"

	"github.com/airplanedev/cli/pkg/build"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
)

// These tests ensure that a View image can be built without error.
// They do not make any assertions on the output of the build, so they aren't great tests.

func TestViewBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []build.Test{
		{
			Root: "view/simple",
			Kind: "view",
			Options: buildtypes.KindOptions{
				"entrypoint": "src/App.tsx",
				"apiHost":    "https://api:5000",
			},
			SkipRun: true,
		},
	}

	build.RunTests(t, ctx, tests)
}

func TestViewBundleBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []build.Test{
		{
			Root: "view/simple",
			Kind: "view",
			Options: buildtypes.KindOptions{
				"apiHost": "https://api:5000",
			},
			SkipRun: true,
			Bundle:  true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.ViewBuildType,
				Version: buildtypes.BuildTypeVersionUnspecified,
			},
			FilesToBuild: []string{
				"src/App.tsx",
			},
		},
		{
			Root: "view/inline",
			Kind: "view",
			Options: buildtypes.KindOptions{
				"apiHost": "https://api:5000",
			},
			SkipRun: true,
			Bundle:  true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.ViewBuildType,
				Version: buildtypes.BuildTypeVersionUnspecified,
			},
			FilesToBuild: []string{
				"src/App.view.tsx",
			},
			FilesToDiscover: []string{
				"src/App.view.tsx",
			},
		},
		{
			Root: "view/inlinemulti",
			Kind: "view",
			Options: buildtypes.KindOptions{
				"apiHost": "https://api:5000",
			},
			SkipRun: true,
			Bundle:  true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.ViewBuildType,
				Version: buildtypes.BuildTypeVersionUnspecified,
			},
			FilesToBuild: []string{
				"src/App.view.tsx",
				"src/App2.view.tsx",
				"src/nested/App.view.tsx",
			},
			FilesToDiscover: []string{
				"src/App.view.tsx",
				"src/App2.view.tsx",
				"src/nested/App.view.tsx",
			},
		},
		{
			Root: "view/yarnworkspaces",
			Kind: "view",
			Options: buildtypes.KindOptions{
				"apiHost": "https://api:5000",
			},
			SkipRun: true,
			Bundle:  true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.ViewBuildType,
				Version: buildtypes.BuildTypeVersionUnspecified,
			},
			FilesToBuild: []string{
				"pkg2/src/main.airplane.tsx",
			},
			FilesToDiscover: []string{
				"pkg2/src/main.airplane.tsx",
			},
		},
		{
			Root: "view/css",
			Kind: "view",
			Options: buildtypes.KindOptions{
				"apiHost": "https://api:5000",
			},
			SkipRun: true,
			Bundle:  true,
			BuildContext: buildtypes.BuildContext{
				Type:    buildtypes.ViewBuildType,
				Version: buildtypes.BuildTypeVersionUnspecified,
			},
			FilesToBuild: []string{
				"myView.airplane.tsx",
			},
			FilesToDiscover: []string{
				"myView.airplane.tsx",
			},
		},
	}

	build.RunTests(t, ctx, tests)
}
