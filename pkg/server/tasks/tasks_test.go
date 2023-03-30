package tasks

import (
	"context"
	"testing"

	libapi "github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/api/cliapi"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/stretchr/testify/require"
)

func TestListTasks(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	taskSlug1 := "task_1"
	taskDefinition := definitions.Definition{
		Name: "My Task",
		Slug: taskSlug1,
		Node: &definitions.NodeDefinition{
			Entrypoint:  "my_task.ts",
			NodeVersion: "18",
		},
	}
	taskDefinition.SetDefnFilePath("my_task.task.yaml")
	taskConfig1 := discover.TaskConfig{
		TaskID:         "tsk1",
		TaskRoot:       ".",
		TaskEntrypoint: "my_task.ts",
		Def:            taskDefinition,
		Source:         discover.ConfigSourceDefn,
	}

	taskSlug2 := "task_2"
	taskDefinition2 := definitions.Definition{
		Name: "My Task 2",
		Slug: taskSlug2,
		Node: &definitions.NodeDefinition{
			Entrypoint:  "my_task_2.ts",
			NodeVersion: "18",
		},
	}
	taskDefinition.SetDefnFilePath("my_task_2.task.yaml")
	taskConfig2 := discover.TaskConfig{
		TaskID:         "tsk2",
		TaskRoot:       ".",
		TaskEntrypoint: "my_task_2.ts",
		Def:            taskDefinition2,
		Source:         discover.ConfigSourceDefn,
	}

	s := &state.State{
		TaskConfigs: state.NewStore[string, discover.TaskConfig](map[string]discover.TaskConfig{
			taskSlug1: taskConfig1,
			taskSlug2: taskConfig2,
		}),
		AppCondition: state.NewStore[string, state.AppCondition](nil),
		DevConfig:    &conf.DevConfig{},
		RemoteClient: &api.MockClient{},
	}

	tasks, err := ListTasks(ctx, s)
	require.NoError(err)

	expectedTask1, err := taskConfigToAPITask(ctx, s, taskConfig1, nil)
	require.NoError(err)

	expectedTask2, err := taskConfigToAPITask(ctx, s, taskConfig2, nil)
	require.NoError(err)

	require.ElementsMatch([]libapi.Task{
		expectedTask1, expectedTask2,
	}, tasks)
}

func TestTaskConfigToAPITask(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()

	taskSlug := "my_task"
	taskDefinition := definitions.Definition{
		Name: "My Task",
		Slug: taskSlug,
		Node: &definitions.NodeDefinition{
			Entrypoint:  "my_task.ts",
			NodeVersion: "18",
		},
		Parameters: []definitions.ParameterDefinition{
			{
				Slug: "param1",
				Type: "shorttext",
			},
			{
				Slug: "param2",
				Type: "shorttext",
			},
		},
	}
	taskDefinition.SetDefnFilePath("my_task.task.yaml")

	tc := discover.TaskConfig{
		TaskID:         "tsk123",
		TaskRoot:       ".",
		TaskEntrypoint: "my_task.ts",
		Def:            taskDefinition,
		Source:         discover.ConfigSourceDefn,
	}

	s := &state.State{
		TaskConfigs: state.NewStore[string, discover.TaskConfig](map[string]discover.TaskConfig{
			taskSlug: tc,
		}),
		AppCondition: state.NewStore[string, state.AppCondition](nil),
		DevConfig:    &conf.DevConfig{},
		RemoteClient: &api.MockClient{},
	}

	task, err := taskConfigToAPITask(ctx, s, tc, nil)
	require.NoError(err)

	require.Equal("tsk123", task.ID)
	require.Equal("My Task", task.Name)
	require.Equal(taskSlug, task.Slug)
	require.Equal([]string{"node"}, task.Command)
	require.Equal(libapi.Parameters{
		libapi.Parameter{
			Slug: "param1",
			Type: "string",
		},
		libapi.Parameter{
			Slug: "param2",
			Type: "string",
		},
	}, task.Parameters)
	require.Equal(buildtypes.TaskKindNode, task.Kind)
	require.Equal(buildtypes.KindOptions{"entrypoint": "my_task.ts", "nodeVersion": "18"}, task.KindOptions)
}
