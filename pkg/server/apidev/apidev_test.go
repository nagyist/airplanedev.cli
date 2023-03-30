package apidev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	libapi "github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/api/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/api/http"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/server/apidev"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/server/test_utils"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/airplanedev/cli/pkg/version"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	require := require.New(t)

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{}, server.Options{}),
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
	taskDefinition := definitions.Definition{
		Name: "My task",
		Slug: taskSlug,
		Node: &definitions.NodeDefinition{
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
						Runtime: buildtypes.TaskRuntimeStandard,
					},
				},
			},
			InitialRemoteEnvSlug: pointers.String("test"),
		}, server.Options{}),
	)

	body := h.GET("/dev/list").
		WithHeader("X-Airplane-Studio-Fallback-Env-Slug", "stage").
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
				Kind: apidev.EntityKindTask,
			},
		},
		"my_view.ts": {
			{
				Name: "My view",
				Slug: "my_view",
				Kind: apidev.EntityKindView,
			},
		},
	}, resp.Entrypoints)
	require.Equal([]apidev.EntityMetadata{
		{
			Name:    "Foo",
			Slug:    "fooslug",
			Kind:    apidev.EntityKindTask,
			Runtime: buildtypes.TaskRuntimeStandard,
		},
	}, resp.RemoteEntrypoints)
}

func getFileSize(require *require.Assertions, path string) *int64 {
	fi, err := os.Stat(path)
	require.NoError(err)
	size := fi.Size()
	return &size
}

func TestListFilesHandler(t *testing.T) {
	require := require.New(t)

	root := "../fixtures/root"
	absoluteDir, err := filepath.Abs(root)
	require.NoError(err)

	taskSlug := "my_task"
	taskDefinition := definitions.Definition{
		Name: "My task",
		Slug: taskSlug,
		Node: &definitions.NodeDefinition{
			Entrypoint:  "my_task.airplane.ts",
			NodeVersion: "18",
		},
	}
	taskDefFileName := filepath.Join(absoluteDir, "my_task.airplane.ts")
	taskDefinition.SetDefnFilePath(taskDefFileName)
	taskDefFileSize := getFileSize(require, taskDefFileName)

	viewSlug := "my_view"
	viewDefFileName := filepath.Join(absoluteDir, "my_view.view.tsx")
	viewDefinition := definitions.ViewDefinition{
		Name:         "My view",
		Entrypoint:   "my_view.view.tsx",
		Slug:         viewSlug,
		DefnFilePath: viewDefFileName,
	}
	viewDefFileSize := getFileSize(require, viewDefFileName)

	subfilePath := filepath.Join(absoluteDir, "subdir/subfile")
	subfileSize := getFileSize(require, subfilePath)

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
		}, server.Options{}),
	)

	body := h.GET("/dev/files/list").
		Expect().
		Status(http.StatusOK).Body()

	var resp apidev.ListFilesResponse
	err = json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal(absoluteDir, resp.Root.Path)
	require.Equal(true, resp.Root.IsDir)
	require.ElementsMatch([]*apidev.FileNode{
		{
			Path: taskDefFileName,
			Size: taskDefFileSize,
			Entities: []apidev.EntityMetadata{
				{
					Name: "My task",
					Slug: "my_task",
					Kind: apidev.EntityKindTask,
				},
			},
			Children: []*apidev.FileNode{},
		},
		{
			Path: viewDefFileName,
			Size: viewDefFileSize,
			Entities: []apidev.EntityMetadata{
				{
					Name: "My view",
					Slug: "my_view",
					Kind: apidev.EntityKindView,
				},
			},
			Children: []*apidev.FileNode{},
		},
		{
			Path:  filepath.Join(absoluteDir, "subdir"),
			IsDir: true,
			Children: []*apidev.FileNode{
				{
					Path:     subfilePath,
					Size:     subfileSize,
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
		}, server.Options{}),
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

	var errResp libhttp.ErrorResponse
	err = json.Unmarshal([]byte(body.Raw()), &errResp)
	require.NoError(err)
	require.Contains(errResp.Error, "no such file or directory")

	// Path outside dev root
	pathOutsideRoot := filepath.Join(absoluteDir, "../my_task.airplane.ts")
	body = h.GET("/dev/files/get").
		WithQuery("path", pathOutsideRoot).
		Expect().
		Status(http.StatusBadRequest).Body()

	err = json.Unmarshal([]byte(body.Raw()), &errResp)
	require.NoError(err)
	require.Contains(errResp.Error, "Path is outside dev root")

	// Traversal elements
	body = h.GET("/dev/files/get").
		WithQuery("path", subfilePath+"/../path").
		Expect().
		Status(http.StatusBadRequest).Body()

	err = json.Unmarshal([]byte(body.Raw()), &errResp)
	require.NoError(err)
	require.Contains(errResp.Error, "Path may not contain directory traversal elements (`..`)")
}

func TestUpdateFileHandler(t *testing.T) {
	require := require.New(t)

	// Create a temporary directory and file
	file, err := os.CreateTemp("", "airplane-dev-test")
	require.NoError(err)
	defer os.RemoveAll(file.Name())

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			Dir: os.TempDir(),
		}, server.Options{}),
	)

	// Valid path
	h.POST("/dev/files/update").
		WithJSON(apidev.UpdateFileRequest{
			Path:    file.Name(),
			Content: "hello",
		}).
		Expect().
		Status(http.StatusOK).Body()

	body := h.GET("/dev/files/get").
		WithQuery("path", file.Name()).
		Expect().
		Status(http.StatusOK).Body()
	var resp apidev.GetFileResponse
	err = json.Unmarshal([]byte(body.Raw()), &resp)
	require.NoError(err)
	require.Equal("hello", resp.Content)

	// Path outside dev root
	body = h.POST("/dev/files/update").
		WithJSON(apidev.UpdateFileRequest{
			Path:    "invalid_path",
			Content: "hello",
		}).
		Expect().
		Status(http.StatusBadRequest).Body()

	var errResp libhttp.ErrorResponse
	err = json.Unmarshal([]byte(body.Raw()), &errResp)
	require.NoError(err)
	require.Contains(errResp.Error, "Path is outside dev root")

	// Traversal elements
	body = h.POST("/dev/files/update").
		WithJSON(apidev.UpdateFileRequest{
			Path:    file.Name() + "/../invalid_path",
			Content: "hello",
		}).
		Expect().
		Status(http.StatusBadRequest).Body()

	err = json.Unmarshal([]byte(body.Raw()), &errResp)
	require.NoError(err)
	require.Contains(errResp.Error, "Path may not contain directory traversal elements (`..`)")
}
