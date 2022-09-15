package discover

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/api/mock"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/stretchr/testify/require"
)

func TestDiscover(t *testing.T) {
	fixturesPath, _ := filepath.Abs("./fixtures")
	tests := []struct {
		name                string
		paths               []string
		existingTasks       map[string]api.Task
		existingViews       map[string]api.View
		expectedErr         bool
		expectedTaskConfigs []TaskConfig
		expectedViewConfigs []ViewConfig
		buildConfigs        []build.BuildConfig
		defnFilePath        string
		absEntrypoints      []string
	}{
		{
			name:  "single script",
			paths: []string{"./fixtures/single_task.js"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "handlebars"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/single_task.js",
					Def: &definitions.Definition_0_3{
						Slug:               "my_task",
						Node:               &definitions.NodeDefinition_0_3{},
						AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
					},
					Source: ConfigSourceScript,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"workdir":    "",
					"entrypoint": "single_task.js",
				},
			},
		},
		{
			name:  "multiple scripts",
			paths: []string{"./fixtures/single_task.js", "./fixtures/single_task2.js"},
			existingTasks: map[string]api.Task{
				"my_task":  {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
				"my_task2": {ID: "tsk456", Slug: "my_task2", Kind: build.TaskKindNode, InterpolationMode: "handlebars"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/single_task.js",
					Def: &definitions.Definition_0_3{
						Slug:               "my_task",
						Node:               &definitions.NodeDefinition_0_3{},
						AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
					},
					Source: ConfigSourceScript,
				},
				{
					TaskID:         "tsk456",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/single_task2.js",
					Def: &definitions.Definition_0_3{
						Slug:               "my_task2",
						Node:               &definitions.NodeDefinition_0_3{},
						AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
					},
					Source: ConfigSourceScript,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"workdir":    "",
					"entrypoint": "single_task.js",
				},
				{
					"workdir":    "",
					"entrypoint": "single_task2.js",
				},
			},
		},
		{
			name:  "nested scripts",
			paths: []string{"./fixtures/nestedScripts"},
			existingTasks: map[string]api.Task{
				"my_task":  {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
				"my_task2": {ID: "tsk456", Slug: "my_task2", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath + "/nestedScripts",
					TaskEntrypoint: fixturesPath + "/nestedScripts/single_task.js",
					Def: &definitions.Definition_0_3{
						Slug:               "my_task",
						Node:               &definitions.NodeDefinition_0_3{},
						AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
					},
					Source: ConfigSourceScript,
				},
				{
					TaskID:         "tsk456",
					TaskRoot:       fixturesPath + "/nestedScripts",
					TaskEntrypoint: fixturesPath + "/nestedScripts/single_task2.js",
					Def: &definitions.Definition_0_3{
						Slug:               "my_task2",
						Node:               &definitions.NodeDefinition_0_3{},
						AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
					},
					Source: ConfigSourceScript,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"workdir":    "",
					"entrypoint": "single_task.js",
				},
				{
					"workdir":    "",
					"entrypoint": "single_task2.js",
				},
			},
		},
		{
			name:  "single defn",
			paths: []string{"./fixtures/defn.task.yaml"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/single_task.js",
					Def: &definitions.Definition_0_3{
						Name:        "sunt in tempor eu",
						Slug:        "my_task",
						Description: "ut dolor sit officia ea",
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint:  "./single_task.js",
							NodeVersion: "14",
						},
					},
					Source: ConfigSourceDefn,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"workdir": "",
				},
			},
			defnFilePath: fixturesPath + "/defn.task.yaml",
			absEntrypoints: []string{
				fixturesPath + "/single_task.js",
			},
		},
		{
			name:          "task not returned by api - deploy skipped",
			paths:         []string{"./fixtures/single_task.js", "./fixtures/defn.task.yaml"},
			existingTasks: map[string]api.Task{},
			expectedErr:   false,
		},
		{
			name:  "defn task archived - deploy skipped",
			paths: []string{"./fixtures/defn.task.yaml"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst", IsArchived: true},
			},
		},
		{
			name:  "script task archived - deploy skipped",
			paths: []string{"./fixtures/single_task.js"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst", IsArchived: true},
			},
		},
		{
			name:  "same task, multiple discoverers",
			paths: []string{"./fixtures/defn.task.yaml", "./fixtures/single_task.js"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/single_task.js",
					Def: &definitions.Definition_0_3{
						Name:        "sunt in tempor eu",
						Slug:        "my_task",
						Description: "ut dolor sit officia ea",
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint:  "./single_task.js",
							NodeVersion: "14",
						},
					},
					Source: ConfigSourceDefn,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"workdir": "",
				},
			},
			defnFilePath: fixturesPath + "/defn.task.yaml",
			absEntrypoints: []string{
				fixturesPath + "/single_task.js",
			},
		},
		{
			name:  "different working directory",
			paths: []string{"./fixtures/subdir/single_task.js"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/subdir/single_task.js",
					Def: &definitions.Definition_0_3{
						Slug:               "my_task",
						Node:               &definitions.NodeDefinition_0_3{},
						AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
					},
					Source: ConfigSourceScript,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"workdir":    "/subdir",
					"entrypoint": "subdir/single_task.js",
				},
			},
		},
		{
			name:  "non linked script with def in same directory",
			paths: []string{"./fixtures/nonlinkedscript/single_task.js"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/nonlinkedscript/single_task.js",
					Def: &definitions.Definition_0_3{
						Name:        "sunt in tempor eu",
						Slug:        "my_task",
						Description: "ut dolor sit officia ea",
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint:  "./single_task.js",
							NodeVersion: "14",
						},
					},
					Source: ConfigSourceDefn,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"workdir":    "/nonlinkedscript",
					"entrypoint": "nonlinkedscript/single_task.js",
				},
			},
			defnFilePath: fixturesPath + "/nonlinkedscript/single_task.task.yaml",
			absEntrypoints: []string{
				fixturesPath + "/nonlinkedscript/single_task.js",
			},
		},
		{
			name:  "non linked script with def in same directory - entire directory deployed",
			paths: []string{"./fixtures/nonlinkedscript"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/nonlinkedscript/single_task.js",
					Def: &definitions.Definition_0_3{
						Name:        "sunt in tempor eu",
						Slug:        "my_task",
						Description: "ut dolor sit officia ea",
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint:  "./single_task.js",
							NodeVersion: "14",
						},
					},
					Source: ConfigSourceDefn,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"workdir":    "/nonlinkedscript",
					"entrypoint": "nonlinkedscript/single_task.js",
				},
			},
			defnFilePath: fixturesPath + "/nonlinkedscript/single_task.task.yaml",
			absEntrypoints: []string{
				fixturesPath + "/nonlinkedscript/single_task.js",
			},
		},
		{
			name:  "discovers definition when script is deployed",
			paths: []string{"./fixtures/subdir/defn.task.yaml"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/subdir/single_task.js",
					Def: &definitions.Definition_0_3{
						Name:        "sunt in tempor eu",
						Slug:        "my_task",
						Description: "ut dolor sit officia ea",
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint:  "./single_task.js",
							NodeVersion: "14",
						},
					},
					Source: ConfigSourceDefn,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"workdir":    "/subdir",
					"entrypoint": "subdir/single_task.js",
				},
			},
			defnFilePath: fixturesPath + "/subdir/defn.task.yaml",
			absEntrypoints: []string{
				fixturesPath + "/subdir/single_task.js",
			},
		},
		{
			name:  "defn - entrypoint does not exist",
			paths: []string{"./fixtures/defn_incorrect_entrypoint.task.yaml"},
			existingTasks: map[string]api.Task{
				"incorrect_entrypoint": {ID: "tsk123", Slug: "incorrect_entrypoint", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedErr: true,
		},
		{
			name:  "view defn",
			paths: []string{"./fixtures/view/defn.view.yaml"},
			existingViews: map[string]api.View{
				"my_view": {ID: "view123", Slug: "my_view", Name: "My View"},
			},
			expectedViewConfigs: []ViewConfig{
				{
					ID: "view123",
					Def: definitions.ViewDefinition{
						Name:         "My View",
						Slug:         "my_view",
						Description:  "Test view yaml file",
						Entrypoint:   fixturesPath + "/view/foo.js",
						DefnFilePath: fixturesPath + "/view/defn.view.yaml",
					},
					Root:   fixturesPath,
					Source: ConfigSourceDefn,
				},
			},
		},
		{
			name:  "python code definition",
			paths: []string{"./fixtures/code_only_task_airplane.py"},
			existingTasks: map[string]api.Task{
				"collatz": {ID: "tsk123", Slug: "collatz", Kind: build.TaskKindPython, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/code_only_task_airplane.py",
					Def: &definitions.Definition_0_3{
						Name:    "Collatz Conjecture Step",
						Slug:    "collatz",
						Timeout: definitions.NewDefaultTimeoutDefinition(3600),
						Parameters: []definitions.ParameterDefinition_0_3{
							{
								Name:     "Num",
								Slug:     "num",
								Type:     "integer",
								Required: definitions.NewDefaultTrueDefinition(true),
								Options:  []definitions.OptionDefinition_0_3{},
							},
						},
						Resources:          definitions.ResourceDefinition_0_3{Attachments: map[string]string{}},
						AllowSelfApprovals: definitions.NewDefaultTrueDefinition(false),
						Python: &definitions.PythonDefinition_0_3{
							Entrypoint: "code_only_task_airplane.py",
						},
						Schedules: map[string]definitions.ScheduleDefinition_0_3{},
					},
					Source: ConfigSourceCode,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"entrypoint":     "code_only_task_airplane.py",
					"entrypointFunc": "collatz",
				},
			},
			absEntrypoints: []string{
				fixturesPath + "/code_only_task_airplane.py",
			},
		},
		{
			name:  "node code definition",
			paths: []string{"./fixtures/codeOnlyTask.airplane.ts"},
			existingTasks: map[string]api.Task{
				"collatz": {ID: "tsk123", Slug: "collatz", Kind: build.TaskKindPython, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/codeOnlyTask.airplane.ts",
					Def: &definitions.Definition_0_3{
						Name: "Collatz Conjecture Step",
						Slug: "collatz",
						Parameters: []definitions.ParameterDefinition_0_3{
							{
								Name: "Num",
								Slug: "num",
								Type: "integer",
							},
						},
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint:  "codeOnlyTask.airplane.ts",
							NodeVersion: "18",
						},
					},
					Source: ConfigSourceCode,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"entrypoint":     "codeOnlyTask.airplane.ts",
					"entrypointFunc": "collatz",
					"workdir":        "",
				},
			},
			absEntrypoints: []string{
				fixturesPath + "/codeOnlyTask.airplane.ts",
			},
		},
		{
			name:  "view code definition",
			paths: []string{"./fixtures/view/codeOnlyView.airplane.tsx"},
			existingViews: map[string]api.View{
				"my_view": {ID: "view123", Slug: "my_view", Name: "My View"},
			},
			expectedViewConfigs: []ViewConfig{
				{
					ID: "view123",
					Def: definitions.ViewDefinition{
						Name:        "My View",
						Slug:        "my_view",
						Description: "Test view yaml file",
						Entrypoint:  fixturesPath + "/view/codeOnlyView.airplane.tsx",
					},
					Root:   fixturesPath,
					Source: ConfigSourceCode,
				},
			},
		},
		{
			name:  "view code definition legacy",
			paths: []string{"./fixtures/view/codeOnly.view.tsx"},
			existingViews: map[string]api.View{
				"my_view": {ID: "view123", Slug: "my_view", Name: "My View"},
			},
			expectedViewConfigs: []ViewConfig{
				{
					ID: "view123",
					Def: definitions.ViewDefinition{
						Name:        "My View",
						Slug:        "my_view",
						Description: "Test view yaml file",
						Entrypoint:  fixturesPath + "/view/codeOnly.view.tsx",
					},
					Root:   fixturesPath,
					Source: ConfigSourceCode,
				},
			},
		},
	}
	for _, tC := range tests {
		t.Run(tC.name, func(t *testing.T) {
			require := require.New(t)
			apiClient := &mock.MockClient{
				Tasks: tC.existingTasks,
				Views: tC.existingViews,
			}
			scriptDiscoverer := &ScriptDiscoverer{
				Client: apiClient,
				Logger: &logger.MockLogger{},
			}
			defnDiscoverer := &DefnDiscoverer{
				Client: apiClient,
				Logger: &logger.MockLogger{},
			}
			codeTaskDiscoverer := &CodeTaskDiscoverer{
				Client: apiClient,
				Logger: &logger.MockLogger{},
			}
			viewDefnDiscoverer := &ViewDefnDiscoverer{
				Client: apiClient,
				Logger: &logger.MockLogger{},
			}
			codeViewDiscoverer := &CodeViewDiscoverer{
				Client: apiClient,
				Logger: &logger.MockLogger{},
			}
			d := &Discoverer{
				TaskDiscoverers: []TaskDiscoverer{defnDiscoverer, scriptDiscoverer, codeTaskDiscoverer},
				ViewDiscoverers: []ViewDiscoverer{viewDefnDiscoverer, codeViewDiscoverer},
				Client:          apiClient,
				Logger:          &logger.MockLogger{},
			}
			taskConfigs, viewConfigs, err := d.Discover(context.Background(), tC.paths...)
			if tC.expectedErr {
				require.NotNil(err)
				return
			}
			require.NoError(err)

			require.Equal(len(tC.expectedTaskConfigs), len(taskConfigs))
			for i := range tC.expectedTaskConfigs {
				for k, v := range tC.buildConfigs[i] {
					tC.expectedTaskConfigs[i].Def.SetBuildConfig(k, v)
				}
				if i < len(tC.absEntrypoints) {
					err := tC.expectedTaskConfigs[i].Def.SetAbsoluteEntrypoint(tC.absEntrypoints[i])
					require.NoError(err)
				}
				tC.expectedTaskConfigs[i].Def.SetDefnFilePath(tC.defnFilePath)
				require.Equal(tC.expectedTaskConfigs[i], taskConfigs[i])
			}

			require.Equal(len(tC.expectedViewConfigs), len(viewConfigs))
			for i := range tC.expectedViewConfigs {
				require.Equal(tC.expectedViewConfigs[i], viewConfigs[i])
			}
		})
	}
}
