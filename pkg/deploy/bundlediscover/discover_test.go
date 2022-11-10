package bundlediscover

import (
	"context"
	"path"
	"path/filepath"
	"testing"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/api/mock"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover(t *testing.T) {
	fixturesPath, _ := filepath.Abs("./fixtures")
	testCases := []struct {
		desc            string
		paths           []string
		existingTasks   map[string]api.Task
		expectedBundles []Bundle
		expectedErr     bool
	}{
		{
			desc:  "task with comment",
			paths: []string{"./fixtures/taskWithComment"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "handlebars"},
			},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"taskWithComment"},
					BuildType:   build.NodeBuildType,
				},
			},
		},
		{
			desc:  "tasks with defn",
			paths: []string{"./fixtures/tasksWithDefn"},
			expectedBundles: []Bundle{
				{
					RootPath:     fixturesPath,
					TargetPaths:  []string{"tasksWithDefn"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode14,
				},
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"tasksWithDefn"},
					BuildType:   build.PythonBuildType,
				},
				{
					RootPath:    path.Join(fixturesPath),
					TargetPaths: []string{"tasksWithDefn"},
					BuildType:   build.ShellBuildType,
				},
				{
					RootPath:    path.Join(fixturesPath),
					TargetPaths: []string{"tasksWithDefn"},
					BuildType:   build.NoneBuildType,
				},
			},
		},
		{
			desc:  "inline tasks",
			paths: []string{"./fixtures/inlineTasks"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"inlineTasks"},
					BuildType:   build.NodeBuildType,
				},
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"inlineTasks"},
					BuildType:   build.PythonBuildType,
				},
			},
		},
		{
			desc:  "inline tasks with version and base set",
			paths: []string{"./fixtures/inlineTasksVersion"},
			expectedBundles: []Bundle{
				{
					RootPath:     path.Join(fixturesPath, "inlineTasksVersion"),
					TargetPaths:  []string{"."},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode16,
					BuildBase:    build.BuildBaseSlim,
				},
			},
		},
		{
			desc:  "non build task (sql, rest, docker)",
			paths: []string{"./fixtures/nonbuildtask"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"nonbuildtask"},
					BuildType:   build.NoneBuildType,
				},
			},
		},
		{
			desc:  "non build task nested ",
			paths: []string{"./fixtures/nonbuildtasknested"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"nonbuildtasknested"},
					BuildType:   build.NoneBuildType,
				},
				{
					RootPath:     fixturesPath,
					TargetPaths:  []string{"nonbuildtasknested/nested/nested"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode18,
				},
			},
		},
		{
			desc:  "view",
			paths: []string{"./fixtures/viewWithDefn"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"viewWithDefn"},
					BuildType:   build.ViewBuildType,
				},
			},
		},
		{
			desc:  "inline view",
			paths: []string{"./fixtures/viewInline"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"viewInline"},
					BuildType:   build.ViewBuildType,
				},
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"viewInline"},
					BuildType:   build.NodeBuildType,
				},
			},
		},
		{
			desc:  "inline view with inline tasks",
			paths: []string{"./fixtures/viewAndTaskInline"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"viewAndTaskInline"},
					BuildType:   build.NodeBuildType,
				},
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"viewAndTaskInline"},
					BuildType:   build.ViewBuildType,
				},
			},
		},
		{
			desc:  "task with defn by script name",
			paths: []string{"./fixtures/tasksWithDefn/defn.js"},
			expectedBundles: []Bundle{
				{
					RootPath:     fixturesPath,
					TargetPaths:  []string{"tasksWithDefn/defn.js"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode14,
				},
			},
		},
		{
			desc:  "multiple paths",
			paths: []string{"./fixtures/inlineTasks", "./fixtures/tasksWithDefn", "./fixtures/tasksWithDefn", "./fixtures/tasksWithDefn/defn.task.yaml"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"inlineTasks"},
					BuildType:   build.NodeBuildType,
				},
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"inlineTasks", "tasksWithDefn"},
					BuildType:   build.PythonBuildType,
				},
				{
					RootPath:     fixturesPath,
					TargetPaths:  []string{"tasksWithDefn", "tasksWithDefn/defn.task.yaml"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode14,
				},
				{
					RootPath:    path.Join(fixturesPath),
					TargetPaths: []string{"tasksWithDefn"},
					BuildType:   build.ShellBuildType,
				},
				{
					RootPath:    path.Join(fixturesPath),
					TargetPaths: []string{"tasksWithDefn"},
					BuildType:   build.NoneBuildType,
				},
			},
		},
		{
			desc:  "task nested in a folder",
			paths: []string{"./fixtures/nestedTask"},
			expectedBundles: []Bundle{
				{
					RootPath:     fixturesPath,
					TargetPaths:  []string{"nestedTask"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode16,
				},
			},
		},
		{
			desc:  "multiple tasks same root same build",
			paths: []string{"./fixtures/multipleTasksSameRoot"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"multipleTasksSameRoot"},
					BuildType:   build.NodeBuildType,
				},
			},
		},
		{
			desc:  "multiple tasks same root diff build",
			paths: []string{"./fixtures/multipleTasksSameRootDiffBuild"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"multipleTasksSameRootDiffBuild"},
					BuildType:   build.NodeBuildType,
				},
				{
					RootPath:     fixturesPath,
					TargetPaths:  []string{"multipleTasksSameRootDiffBuild"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode16,
				},
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"multipleTasksSameRootDiffBuild"},
					BuildType:   build.NodeBuildType,
					BuildBase:   build.BuildBaseSlim,
				},
			},
		},
		{
			desc:  "multiple tasks diff root",
			paths: []string{"./fixtures/multipleTasksDiffRoot"},
			expectedBundles: []Bundle{
				{
					RootPath:     fixturesPath,
					TargetPaths:  []string{"multipleTasksDiffRoot"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode14,
				},
				{
					RootPath:     path.Join(fixturesPath),
					TargetPaths:  []string{"multipleTasksDiffRoot/nested"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode14,
				},
				{
					RootPath:     path.Join(fixturesPath),
					TargetPaths:  []string{"multipleTasksDiffRoot/nested/nested"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode14,
				},
			},
		},
		{
			desc:            "no entities match paths",
			paths:           []string{"./discover_test.go"},
			expectedBundles: []Bundle{},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)
			apiClient := &mock.MockClient{
				Tasks: tC.existingTasks,
			}
			scriptDiscoverer := &discover.ScriptDiscoverer{
				Client: apiClient,
				Logger: &logger.MockLogger{},
			}
			defnDiscoverer := &discover.DefnDiscoverer{
				Client: apiClient,
				Logger: &logger.MockLogger{},
			}
			codeTaskDiscoverer := &discover.CodeTaskDiscoverer{
				Client: apiClient,
				Logger: &logger.MockLogger{},
			}
			viewDefnDiscoverer := &discover.ViewDefnDiscoverer{
				Client: apiClient,
				Logger: &logger.MockLogger{},
			}
			codeViewDiscoverer := &discover.CodeViewDiscoverer{
				Client: apiClient,
				Logger: &logger.MockLogger{},
			}
			d := &Discoverer{
				TaskDiscoverers: []discover.TaskDiscoverer{defnDiscoverer, scriptDiscoverer, codeTaskDiscoverer},
				ViewDiscoverers: []discover.ViewDiscoverer{viewDefnDiscoverer, codeViewDiscoverer},
				Client:          apiClient,
				Logger:          &logger.MockLogger{},
			}

			bundles, err := d.Discover(context.Background(), tC.paths...)
			if tC.expectedErr {
				require.NotNil(err)
				return
			}
			require.NoError(err)

			assert.ElementsMatch(tC.expectedBundles, bundles)

		})
	}
}
