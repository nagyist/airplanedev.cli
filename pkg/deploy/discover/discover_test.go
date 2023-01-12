package discover

import (
	"context"
	"path"
	"path/filepath"
	"testing"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/api/mock"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/airplanedev/lib/pkg/utils/pointers"
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
		defnFilePaths       []string
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
						Parameters:         []definitions.ParameterDefinition_0_3{},
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
						Parameters:         []definitions.ParameterDefinition_0_3{},
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
						Parameters:         []definitions.ParameterDefinition_0_3{},
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
						Parameters:         []definitions.ParameterDefinition_0_3{},
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
						Parameters:         []definitions.ParameterDefinition_0_3{},
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
							Entrypoint: "./single_task.js",
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
			defnFilePaths: []string{fixturesPath + "/defn.task.yaml"},
			absEntrypoints: []string{
				fixturesPath + "/single_task.js",
			},
		},
		{
			name:  "task definitions with version in bundle",
			paths: []string{"./fixtures/tasksWithVersion"},
			existingTasks: map[string]api.Task{
				"my_task":  {ID: "tsk121", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
				"my_task2": {ID: "tsk122", Slug: "my_task2", Kind: build.TaskKindNode, InterpolationMode: "jst"},
				"my_task3": {ID: "tsk123", Slug: "my_task3", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk121",
					TaskRoot:       path.Join(fixturesPath, "tasksWithVersion", "18"),
					TaskEntrypoint: path.Join(fixturesPath, "tasksWithVersion", "18", "node.ts"),
					Def: &definitions.Definition_0_3{
						Name: "my_task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint:  "node.ts",
							NodeVersion: "18",
						},
					},
					Source: ConfigSourceDefn,
				},
				{
					TaskID:         "tsk122",
					TaskRoot:       path.Join(fixturesPath, "tasksWithVersion", "gt17"),
					TaskEntrypoint: path.Join(fixturesPath, "tasksWithVersion", "gt17", "node.ts"),
					Def: &definitions.Definition_0_3{
						Name: "my_task2",
						Slug: "my_task2",
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint:  "node.ts",
							NodeVersion: "18",
						},
					},
					Source: ConfigSourceDefn,
				},
				{
					TaskID:         "tsk123",
					TaskRoot:       path.Join(fixturesPath, "tasksWithVersion", "lt18gt14"),
					TaskEntrypoint: path.Join(fixturesPath, "tasksWithVersion", "lt18gt14", "node.ts"),
					Def: &definitions.Definition_0_3{
						Name: "my_task3",
						Slug: "my_task3",
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint:  "node.ts",
							NodeVersion: "16",
						},
					},
					Source: ConfigSourceDefn,
				},
			},
			buildConfigs: []build.BuildConfig{
				{"workdir": ""},
				{"workdir": ""},
				{"workdir": ""},
			},
			defnFilePaths: []string{
				path.Join(fixturesPath, "tasksWithVersion", "18", "node.task.yaml"),
				path.Join(fixturesPath, "tasksWithVersion", "gt17", "node.task.yaml"),
				path.Join(fixturesPath, "tasksWithVersion", "lt18gt14", "node.task.yaml"),
			},
			absEntrypoints: []string{
				path.Join(fixturesPath, "tasksWithVersion", "18", "node.ts"),
				path.Join(fixturesPath, "tasksWithVersion", "gt17", "node.ts"),
				path.Join(fixturesPath, "tasksWithVersion", "lt18gt14", "node.ts"),
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
							Entrypoint: "./single_task.js",
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
			defnFilePaths: []string{fixturesPath + "/defn.task.yaml"},
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
						Parameters:         []definitions.ParameterDefinition_0_3{},
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
			defnFilePaths: []string{fixturesPath + "/nonlinkedscript/single_task.task.yaml"},
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
			defnFilePaths: []string{fixturesPath + "/nonlinkedscript/single_task.task.yaml"},
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
			defnFilePaths: []string{fixturesPath + "/subdir/defn.task.yaml"},
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
						Base:         build.BuildBaseSlim,
					},
					Root:   fixturesPath,
					Source: ConfigSourceDefn,
				},
			},
		},
		{
			name:  "python code definition",
			paths: []string{"./fixtures/taskInline/python/task_a_airplane.py"},
			existingTasks: map[string]api.Task{
				"task_a": {ID: "tsk123", Slug: "task_a", Kind: build.TaskKindPython, InterpolationMode: "jst"},
				"task_b": {ID: "tsk123", Slug: "task_b", Kind: build.TaskKindPython, InterpolationMode: "jst"},
				"task_c": {ID: "tsk123", Slug: "task_c", Kind: build.TaskKindPython, InterpolationMode: "jst"},
				"task_d": {ID: "tsk123", Slug: "task_d", Kind: build.TaskKindPython, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath + "/taskInline/python",
					TaskEntrypoint: fixturesPath + "/taskInline/python/task_a_airplane.py",
					Def: &definitions.Definition_0_3{
						Name:    "Task A",
						Slug:    "task_a",
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
							EnvVars:    api.TaskEnv{},
							Entrypoint: "task_a_airplane.py",
						},
						Schedules: map[string]definitions.ScheduleDefinition_0_3{},
					},
					Source: ConfigSourceCode,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"entrypoint":     "task_a_airplane.py",
					"entrypointFunc": "task_a",
				},
			},
			absEntrypoints: []string{
				fixturesPath + "/taskInline/python/task_a_airplane.py",
			},
			defnFilePaths: []string{fixturesPath + "/taskInline/python/task_a_airplane.py"},
		},
		{
			name:  "python code definition import",
			paths: []string{"./fixtures/taskInline/python/task_b_airplane.py"},
			existingTasks: map[string]api.Task{
				"task_a": {ID: "tsk123", Slug: "task_a", Kind: build.TaskKindPython, InterpolationMode: "jst"},
				"task_b": {ID: "tsk123", Slug: "task_b", Kind: build.TaskKindPython, InterpolationMode: "jst"},
				"task_c": {ID: "tsk123", Slug: "task_c", Kind: build.TaskKindPython, InterpolationMode: "jst"},
				"task_d": {ID: "tsk123", Slug: "task_d", Kind: build.TaskKindPython, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath + "/taskInline/python",
					TaskEntrypoint: fixturesPath + "/taskInline/python/task_b_airplane.py",
					Def: &definitions.Definition_0_3{
						Name:    "Task B",
						Slug:    "task_b",
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
							EnvVars:    api.TaskEnv{},
							Entrypoint: "task_b_airplane.py",
						},
						Schedules: map[string]definitions.ScheduleDefinition_0_3{},
					},
					Source: ConfigSourceCode,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"entrypoint":     "task_b_airplane.py",
					"entrypointFunc": "task_b",
				},
			},
			absEntrypoints: []string{
				fixturesPath + "/taskInline/python/task_b_airplane.py",
			},
			defnFilePaths: []string{fixturesPath + "/taskInline/python/task_b_airplane.py"},
		},
		{
			name:  "python code definition multiple import",
			paths: []string{"./fixtures/taskInline/python/task_c_airplane.py"},
			existingTasks: map[string]api.Task{
				"task_a": {ID: "tsk123", Slug: "task_a", Kind: build.TaskKindPython, InterpolationMode: "jst"},
				"task_b": {ID: "tsk123", Slug: "task_b", Kind: build.TaskKindPython, InterpolationMode: "jst"},
				"task_c": {ID: "tsk123", Slug: "task_c", Kind: build.TaskKindPython, InterpolationMode: "jst"},
				"task_d": {ID: "tsk123", Slug: "task_d", Kind: build.TaskKindPython, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath + "/taskInline/python",
					TaskEntrypoint: fixturesPath + "/taskInline/python/task_c_airplane.py",
					Def: &definitions.Definition_0_3{
						Name:    "Task C",
						Slug:    "task_c",
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
							EnvVars:    api.TaskEnv{},
							Entrypoint: "task_c_airplane.py",
						},
						Schedules: map[string]definitions.ScheduleDefinition_0_3{},
					},
					Source: ConfigSourceCode,
				},
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath + "/taskInline/python",
					TaskEntrypoint: fixturesPath + "/taskInline/python/task_c_airplane.py",
					Def: &definitions.Definition_0_3{
						Name:    "Task D",
						Slug:    "task_d",
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
							EnvVars:    api.TaskEnv{},
							Entrypoint: "task_c_airplane.py",
						},
						Schedules: map[string]definitions.ScheduleDefinition_0_3{},
					},
					Source: ConfigSourceCode,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"entrypoint":     "task_c_airplane.py",
					"entrypointFunc": "task_c",
				},
				{
					"entrypoint":     "task_c_airplane.py",
					"entrypointFunc": "task_d",
				},
			},
			absEntrypoints: []string{
				fixturesPath + "/taskInline/python/task_c_airplane.py",
				fixturesPath + "/taskInline/python/task_c_airplane.py",
			},
			defnFilePaths: []string{
				fixturesPath + "/taskInline/python/task_c_airplane.py",
				fixturesPath + "/taskInline/python/task_c_airplane.py",
			},
		},
		{
			name:  "node code definition",
			paths: []string{"./fixtures/taskInline/codeOnlyTask.airplane.ts"},
			existingTasks: map[string]api.Task{
				"collatz": {ID: "tsk123", Slug: "collatz", Kind: build.TaskKindPython, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/taskInline/codeOnlyTask.airplane.ts",
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
							Entrypoint: "taskInline/codeOnlyTask.airplane.ts",
						},
					},
					Source: ConfigSourceCode,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"entrypoint":     "taskInline/codeOnlyTask.airplane.ts",
					"entrypointFunc": "collatz",
					"workdir":        "",
				},
			},
			absEntrypoints: []string{
				fixturesPath + "/taskInline/codeOnlyTask.airplane.ts",
			},
			defnFilePaths: []string{fixturesPath + "/taskInline/codeOnlyTask.airplane.ts"},
		},
		{
			name:  "node code definition with an esm dep",
			paths: []string{"./fixtures/taskInlineEsm/codeOnlyTask.airplane.ts"},
			existingTasks: map[string]api.Task{
				"collatz": {ID: "tsk123", Slug: "collatz", Kind: build.TaskKindPython, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/taskInlineEsm/codeOnlyTask.airplane.ts",
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
							Entrypoint: "taskInlineEsm/codeOnlyTask.airplane.ts",
						},
					},
					Source: ConfigSourceCode,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"entrypoint":     "taskInlineEsm/codeOnlyTask.airplane.ts",
					"entrypointFunc": "collatz",
					"workdir":        "",
				},
			},
			absEntrypoints: []string{
				fixturesPath + "/taskInlineEsm/codeOnlyTask.airplane.ts",
			},
			defnFilePaths: []string{fixturesPath + "/taskInlineEsm/codeOnlyTask.airplane.ts"},
		},
		{
			name:  "node code definition with env vars in code and in config file",
			paths: []string{"./fixtures/envvars/codeOnlyTask.airplane.ts"},
			existingTasks: map[string]api.Task{
				"collatz": {ID: "tsk123", Slug: "collatz", Kind: build.TaskKindPython, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath + "/envvars",
					TaskEntrypoint: fixturesPath + "/envvars/codeOnlyTask.airplane.ts",
					Def: &definitions.Definition_0_3{
						Name:       "Collatz Conjecture Step",
						Slug:       "collatz",
						Parameters: []definitions.ParameterDefinition_0_3{},
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint: "codeOnlyTask.airplane.ts",
							EnvVars: api.TaskEnv{
								"ENV1": api.EnvVarValue{Value: pointers.String("1")},
								"ENV2": api.EnvVarValue{Value: pointers.String("2")},
								"ENV3": api.EnvVarValue{Value: pointers.String("3a")},
							},
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
				fixturesPath + "/envvars/codeOnlyTask.airplane.ts",
			},
			defnFilePaths: []string{fixturesPath + "/envvars/codeOnlyTask.airplane.ts"},
		},
		{
			name:  "single defn with env vars in defn and in config file",
			paths: []string{"./fixtures/envvars/defn.task.yaml"},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath + "/envvars",
					TaskEntrypoint: fixturesPath + "/envvars/single_task.js",
					Def: &definitions.Definition_0_3{
						Name:        "sunt in tempor eu",
						Slug:        "my_task",
						Description: "ut dolor sit officia ea",
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint: "./single_task.js",
							EnvVars: api.TaskEnv{
								"ENV2": api.EnvVarValue{Value: pointers.String("2")},
								"ENV3": api.EnvVarValue{Value: pointers.String("3a")},
								"ENV5": api.EnvVarValue{Value: pointers.String("5")},
							},
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
			absEntrypoints: []string{
				fixturesPath + "/envvars/single_task.js",
			},
			defnFilePaths: []string{fixturesPath + "/envvars/defn.task.yaml"},
		},
		{
			name:  "view code definition",
			paths: []string{"./fixtures/viewInline/myView/myView.view.tsx"},
			existingViews: map[string]api.View{
				"my_view": {ID: "view123", Slug: "my_view", Name: "My View"},
			},
			expectedViewConfigs: []ViewConfig{
				{
					ID: "view123",
					Def: definitions.ViewDefinition{
						Name:         "My View",
						Slug:         "my_view",
						Description:  "my description",
						Entrypoint:   fixturesPath + "/viewInline/myView/myView.view.tsx",
						DefnFilePath: fixturesPath + "/viewInline/myView/myView.view.tsx",
					},
					Root:   fixturesPath,
					Source: ConfigSourceCode,
				},
			},
		},
		{
			name:  "view code definition with task definition",
			paths: []string{"./fixtures/viewInline-with-tasks/myView/myView.view.tsx"},
			existingViews: map[string]api.View{
				"my_view": {ID: "view123", Slug: "my_view", Name: "My View"},
			},
			existingTasks: map[string]api.Task{
				"my_task": {ID: "tsk123", Slug: "my_task", Kind: build.TaskKindNode, InterpolationMode: "jst"},
			},
			expectedViewConfigs: []ViewConfig{
				{
					ID: "view123",
					Def: definitions.ViewDefinition{
						Name:         "My View",
						Slug:         "my_view",
						Description:  "my description",
						Entrypoint:   fixturesPath + "/viewInline-with-tasks/myView/myView.view.tsx",
						DefnFilePath: fixturesPath + "/viewInline-with-tasks/myView/myView.view.tsx",
					},
					Root:   fixturesPath,
					Source: ConfigSourceCode,
				},
			},
			expectedTaskConfigs: []TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: fixturesPath + "/viewInline-with-tasks/myView/myView.view.tsx",
					Def: &definitions.Definition_0_3{
						Name:       "My Task",
						Slug:       "my_task",
						Parameters: []definitions.ParameterDefinition_0_3{},
						Node: &definitions.NodeDefinition_0_3{
							Entrypoint: "viewInline-with-tasks/myView/myView.view.tsx",
						},
						Description: "my description",
					},
					Source: ConfigSourceCode,
				},
			},
			buildConfigs: []build.BuildConfig{
				{
					"entrypoint":     "viewInline-with-tasks/myView/myView.view.tsx",
					"entrypointFunc": "myTask",
					"workdir":        "",
				},
			},
			absEntrypoints: []string{
				fixturesPath + "/viewInline-with-tasks/myView/myView.view.tsx",
			},
			defnFilePaths: []string{fixturesPath + "/viewInline-with-tasks/myView/myView.view.tsx"},
		},
		{
			name:  "view code definition - airplane.tsx",
			paths: []string{"./fixtures/viewInline-airplanetsx/myView/myView.airplane.tsx"},
			existingViews: map[string]api.View{
				"my_view": {ID: "view123", Slug: "my_view", Name: "My View"},
			},
			expectedViewConfigs: []ViewConfig{
				{
					ID: "view123",
					Def: definitions.ViewDefinition{
						Name:         "My View",
						Slug:         "my_view",
						Description:  "hi",
						Entrypoint:   fixturesPath + "/viewInline-airplanetsx/myView/myView.airplane.tsx",
						DefnFilePath: fixturesPath + "/viewInline-airplanetsx/myView/myView.airplane.tsx",
					},
					Root:   fixturesPath,
					Source: ConfigSourceCode,
				},
			},
		},
		{
			name:  "view code definition that imports css",
			paths: []string{"./fixtures/viewInlineCSS/myView/myView.view.tsx"},
			existingViews: map[string]api.View{
				"my_view": {ID: "view123", Slug: "my_view", Name: "My View"},
			},
			expectedViewConfigs: []ViewConfig{
				{
					ID: "view123",
					Def: definitions.ViewDefinition{
						Name:         "My View",
						Slug:         "my_view",
						Description:  "my description",
						Entrypoint:   fixturesPath + "/viewInlineCSS/myView/myView.view.tsx",
						DefnFilePath: fixturesPath + "/viewInlineCSS/myView/myView.view.tsx",
					},
					Root:   fixturesPath + "/viewInlineCSS",
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
				if i < len(tC.defnFilePaths) {
					tC.expectedTaskConfigs[i].Def.SetDefnFilePath(tC.defnFilePaths[i])
				}
				require.Equal(tC.expectedTaskConfigs[i], taskConfigs[i])
			}

			require.Equal(len(tC.expectedViewConfigs), len(viewConfigs))
			for i := range tC.expectedViewConfigs {
				require.Equal(tC.expectedViewConfigs[i], viewConfigs[i])
			}
		})
	}
}
