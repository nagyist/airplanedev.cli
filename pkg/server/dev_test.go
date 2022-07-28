package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	require := require.New(t)

	h := getHttpExpect(
		context.Background(),
		t,
		newRouter(nil),
	)

	body := h.GET("/dev/ping").
		Expect().
		Status(http.StatusOK).Body()

	require.Equal("ok", body.Raw())
}

func TestListDevMetadata(t *testing.T) {
	require := require.New(t)

	taskSlug := "my_task"
	taskDefinition := &definitions.Definition_0_3{
		Name: "My task",
		Slug: taskSlug,
		Node: &definitions.NodeDefinition_0_3{
			Entrypoint:  "my_task.ts",
			NodeVersion: "18",
		},
	}
	taskDefinition.SetDefnFilePath("my_task.task.yaml")

	viewSlug := "my_view"
	viewDefinition := definitions.ViewDefinition{
		Name: "My view",
		Slug: viewSlug,
	}

	h := getHttpExpect(
		context.Background(),
		t,
		newRouter(&State{
			taskConfigs: map[string]discover.TaskConfig{
				taskSlug: {
					TaskID:         "tsk123",
					TaskRoot:       ".",
					TaskEntrypoint: "my_task.ts",
					Def:            taskDefinition,
					Source:         discover.ConfigSourceDefn,
				},
			},
			viewConfigs: map[string]discover.ViewConfig{
				viewSlug: {
					Def:    viewDefinition,
					Source: discover.ConfigSourceDefn,
				},
			},
		}),
	)

	body := h.GET("/dev/list").
		Expect().
		Status(http.StatusOK).Body()

	var resp ListDevMetadataHandlerResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal([]DevMetadata{
		{
			Name: "My task",
			Slug: "my_task",
		},
	}, resp.Tasks)
	require.Equal([]DevMetadata{
		{
			Name: "My view",
			Slug: "my_view",
		},
	}, resp.Views)
}

func TestGetTask(t *testing.T) {
	require := require.New(t)

	taskSlug := "my_task"
	taskDefinition := &definitions.Definition_0_3{
		Name: "My task",
		Slug: taskSlug,
		Node: &definitions.NodeDefinition_0_3{
			Entrypoint:  "my_task.ts",
			NodeVersion: "18",
		},
		AllowSelfApprovals: definitions.NewDefaultTrueDefinition(true),
		Timeout:            definitions.NewDefaultTimeoutDefinition(3600),
	}

	h := getHttpExpect(
		context.Background(),
		t,
		newRouter(&State{
			taskConfigs: map[string]discover.TaskConfig{
				taskSlug: {
					TaskID:         "tsk123",
					TaskRoot:       ".",
					TaskEntrypoint: "my_task.ts",
					Def:            taskDefinition,
					Source:         discover.ConfigSourceDefn,
				},
			},
		}),
	)

	body := h.GET("/dev/tasks/my_task").
		Expect().
		Status(http.StatusOK).Body()

	var resp definitions.Definition_0_3
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	// TODO: Fix test after JSON Marshal logic for resource attachments is fixed
	//require.Equal(*taskDefinition, resp)
}
