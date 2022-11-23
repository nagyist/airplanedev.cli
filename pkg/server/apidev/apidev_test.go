package apidev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/server/apidev"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/server/test_utils"
	"github.com/airplanedev/cli/pkg/version"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	require := require.New(t)

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{}),
	)

	body := h.GET("/dev/version").
		Expect().
		Status(http.StatusOK).Body()

	var resp version.Metadata
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)

	require.Equal(resp.Status, "ok")
	require.NotEmpty(resp.Status)

}

func TestListEntrypoints(t *testing.T) {
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
		Name:       "My view",
		Entrypoint: "my_view.ts",
		Slug:       viewSlug,
	}

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			TaskConfigs: state.NewStore(map[string]discover.TaskConfig{
				taskSlug: {
					TaskID:         "tsk123",
					TaskRoot:       ".",
					TaskEntrypoint: "my_task.ts",
					Def:            taskDefinition,
					Source:         discover.ConfigSourceDefn,
				},
			}),
			ViewConfigs: state.NewStore(map[string]discover.ViewConfig{
				viewSlug: {
					Def:    viewDefinition,
					Source: discover.ConfigSourceDefn,
				},
			}),
			RemoteClient: &api.MockClient{
				Tasks: map[string]libapi.Task{
					"fooslug": {
						Name:    "Foo",
						Slug:    "fooslug",
						Runtime: build.TaskRuntimeStandard,
					},
				},
			},
			UseFallbackEnv: true,
		}),
	)

	body := h.GET("/dev/list").
		Expect().
		Status(http.StatusOK).Body()

	var resp apidev.ListEntrypointsHandlerResponse
	err := json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(map[string][]apidev.EntityMetadata{
		"my_task.ts": {
			{
				Name: "My task",
				Slug: "my_task",
				Kind: apidev.AppKindTask,
			},
		},
		"my_view.ts": {
			{
				Name: "My view",
				Slug: "my_view",
				Kind: apidev.AppKindView,
			},
		},
	}, resp.Entrypoints)
	require.Equal([]apidev.EntityMetadata{
		{
			Name:    "Foo",
			Slug:    "fooslug",
			Kind:    apidev.AppKindTask,
			Runtime: build.TaskRuntimeStandard,
		},
	}, resp.RemoteEntrypoints)
}

func TestListFilesHandler(t *testing.T) {
	require := require.New(t)

	root := "../fixtures/root"
	absoluteDir, err := filepath.Abs(root)
	require.NoError(err)

	taskSlug := "my_task"
	taskDefinition := &definitions.Definition_0_3{
		Name: "My task",
		Slug: taskSlug,
		Node: &definitions.NodeDefinition_0_3{
			Entrypoint:  "my_task.airplane.ts",
			NodeVersion: "18",
		},
	}
	taskDefFileName := filepath.Join(absoluteDir, "my_task.airplane.ts")
	taskDefinition.SetDefnFilePath(taskDefFileName)

	viewSlug := "my_view"
	viewDefFileName := filepath.Join(absoluteDir, "my_view.view.tsx")
	viewDefinition := definitions.ViewDefinition{
		Name:         "My view",
		Entrypoint:   "my_view.view.tsx",
		Slug:         viewSlug,
		DefnFilePath: viewDefFileName,
	}

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			TaskConfigs: state.NewStore(map[string]discover.TaskConfig{
				taskSlug: {
					TaskID:         "tsk123",
					TaskRoot:       ".",
					TaskEntrypoint: "my_task.airplane.ts",
					Def:            taskDefinition,
					Source:         discover.ConfigSourceDefn,
				},
			}),
			ViewConfigs: state.NewStore(map[string]discover.ViewConfig{
				viewSlug: {
					Def:    viewDefinition,
					Source: discover.ConfigSourceDefn,
				},
			}),
			Dir: absoluteDir,
		}),
	)

	body := h.GET("/dev/files/list").
		Expect().
		Status(http.StatusOK).Body()

	var resp apidev.ListFilesResponse
	err = json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(absoluteDir, resp.Root.Path)
	require.ElementsMatch([]*apidev.FileNode{
		{
			Path: taskDefFileName,
			Entities: []apidev.EntityMetadata{
				{
					Name: "My task",
					Slug: "my_task",
					Kind: apidev.AppKindTask,
				},
			},
			Children: []*apidev.FileNode{},
		},
		{
			Path: viewDefFileName,
			Entities: []apidev.EntityMetadata{
				{
					Name: "My view",
					Slug: "my_view",
					Kind: apidev.AppKindView,
				},
			},
			Children: []*apidev.FileNode{},
		},
		{
			Path: filepath.Join(absoluteDir, "subdir"),
			Children: []*apidev.FileNode{
				{
					Path:     filepath.Join(absoluteDir, "subdir/subfile"),
					Children: []*apidev.FileNode{},
				},
			},
		},
	}, resp.Root.Children)
}

func TestGetFileHandler(t *testing.T) {
	require := require.New(t)

	root := "../fixtures/root/subdir"
	absoluteDir, err := filepath.Abs(root)
	require.NoError(err)

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			Dir: absoluteDir,
		}),
	)

	subfilePath := filepath.Join(absoluteDir, "subfile")

	// Valid path
	body := h.GET("/dev/files/get").
		WithQuery("path", subfilePath).
		Expect().
		Status(http.StatusOK).Body()

	var resp apidev.GetFileResponse
	err = json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal("Test\n", resp.Content)

	// Nonexistent path
	nonexistentPath := filepath.Join(absoluteDir, "nonexistent")
	body = h.GET("/dev/files/get").
		WithQuery("path", nonexistentPath).
		Expect().
		Status(http.StatusInternalServerError).Body()

	var errResp test_utils.ErrorResponse
	err = json.Unmarshal([]byte(body.Raw()), &errResp)
	require.NoError(err)
	require.Contains(errResp.Error, "no such file or directory")

	// Path outside dev root
	pathOutsideRoot := filepath.Join(absoluteDir, "../my_task.airplane.ts")
	body = h.GET("/dev/files/get").
		WithQuery("path", pathOutsideRoot).
		Expect().
		Status(http.StatusInternalServerError).Body()

	err = json.Unmarshal([]byte(body.Raw()), &errResp)
	require.NoError(err)
	require.Contains(errResp.Error, "path is outside dev root")
}
