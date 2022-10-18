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
				},
			},
		},
		{
			desc:  "task with defn",
			paths: []string{"./fixtures/taskWithDefn"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"taskWithDefn"},
				},
			},
		},
		{
			desc:  "inline task",
			paths: []string{"./fixtures/inlineTask"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"inlineTask"},
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
				},
			},
		},
		{
			desc:  "task with defn by script name",
			paths: []string{"./fixtures/taskWithDefn/defn.js"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"taskWithDefn/defn.js"},
				},
			},
		},
		{
			desc:  "multiple paths",
			paths: []string{"./fixtures/inlineTask", "./fixtures/taskWithDefn", "./fixtures/taskWithDefn", "./fixtures/taskWithDefn/defn.task.yaml"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"inlineTask", "taskWithDefn"},
				},
			},
		},
		{
			desc:  "task nested in a folder",
			paths: []string{"./fixtures/nestedTask"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"nestedTask"},
				},
			},
		},
		{
			desc:  "multiple tasks same root",
			paths: []string{"./fixtures/multipleTasksSameRoot"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"multipleTasksSameRoot"},
				},
			},
		},
		{
			desc:  "multiple tasks diff root",
			paths: []string{"./fixtures/multipleTasksDiffRoot"},
			expectedBundles: []Bundle{
				{
					RootPath:    fixturesPath,
					TargetPaths: []string{"multipleTasksDiffRoot"},
				},
				{
					RootPath:    path.Join(fixturesPath, "multipleTasksDiffRoot/nested"),
					TargetPaths: []string{"."},
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
