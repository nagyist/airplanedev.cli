package build

import (
	"context"
	"testing"
)

// These tests ensure that a View image can be built without error.
// They do not make any assertions on the output of the build, so they aren't great tests.

func TestViewBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []Test{
		{
			Root: "view/simple",
			Kind: "view",
			Options: KindOptions{
				"entrypoint": "src/App.tsx",
				"apiHost":    "https://api:5000",
			},
			SkipRun: true,
		},
	}

	RunTests(t, ctx, tests)
}

func TestViewBundleBuilder(t *testing.T) {
	ctx := context.Background()

	tests := []Test{
		{
			Root: "view/simple",
			Kind: "view",
			Options: KindOptions{
				"apiHost": "https://api:5000",
			},
			SkipRun: true,
			Bundle:  true,
			BuildContext: BuildContext{
				Type:    ViewBuildType,
				Version: BuildTypeVersionUnspecified,
			},
			FilesToBuild: []string{
				"src/App.tsx",
			},
		},
		{
			Root: "view/inline",
			Kind: "view",
			Options: KindOptions{
				"apiHost": "https://api:5000",
			},
			SkipRun: true,
			Bundle:  true,
			BuildContext: BuildContext{
				Type:    ViewBuildType,
				Version: BuildTypeVersionUnspecified,
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
			Options: KindOptions{
				"apiHost": "https://api:5000",
			},
			SkipRun: true,
			Bundle:  true,
			BuildContext: BuildContext{
				Type:    ViewBuildType,
				Version: BuildTypeVersionUnspecified,
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
			Root: "view/css",
			Kind: "view",
			Options: KindOptions{
				"apiHost": "https://api:5000",
			},
			SkipRun: true,
			Bundle:  true,
			BuildContext: BuildContext{
				Type:    ViewBuildType,
				Version: BuildTypeVersionUnspecified,
			},
			FilesToBuild: []string{
				"myView.airplane.tsx",
			},
			FilesToDiscover: []string{
				"myView.airplane.tsx",
			},
		},
	}

	RunTests(t, ctx, tests)
}
