package apidev

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"

	libhttp "github.com/airplanedev/cli/pkg/api/http"
	"github.com/airplanedev/cli/pkg/build/ignore"
	"github.com/airplanedev/cli/pkg/server/files"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
)

// ListFilesHandler handles requests to the /dev/files/list endpoint. It generates a tree of all files under the dev
// server root, with entities that are declared in each file.
func ListFilesHandler(ctx context.Context, state *state.State, r *http.Request) (ListFilesResponse, error) {
	// Track entities per file, which we'll use to show entities in the UI.
	filepathToEntities := make(map[string][]EntityMetadata, 0)
	for slug, taskConfig := range state.TaskConfigs.Items() {
		defnFilePath := taskConfig.Def.GetDefnFilePath()
		if _, ok := filepathToEntities[defnFilePath]; !ok {
			filepathToEntities[defnFilePath] = make([]EntityMetadata, 0, 1)
		}
		filepathToEntities[defnFilePath] = append(filepathToEntities[defnFilePath], EntityMetadata{
			Name:    taskConfig.Def.GetName(),
			Slug:    slug,
			Kind:    EntityKindTask,
			Runtime: taskConfig.Def.GetRuntime(),
		})
	}

	for slug, viewConfig := range state.ViewConfigs.Items() {
		defnFilePath := viewConfig.Def.DefnFilePath
		if _, ok := filepathToEntities[defnFilePath]; !ok {
			filepathToEntities[defnFilePath] = make([]EntityMetadata, 0, 1)
		}
		filepathToEntities[defnFilePath] = append(filepathToEntities[defnFilePath], EntityMetadata{
			Name: viewConfig.Def.Name,
			Slug: slug,
			Kind: EntityKindView,
		})
	}

	sortEntityMap(filepathToEntities)

	// Track all file tree nodes. We'll use this to build the file tree. Inspired by https://github.com/marcinwyszynski/directory_tree
	nodes := make(map[string]*FileNode)
	if err := filepath.Walk(state.Dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Ignore non-user-facing directories.
			var isDir bool
			var size *int64
			if info.IsDir() {
				base := filepath.Base(path)
				if _, ok := ignoredDirs[base]; ok {
					return filepath.SkipDir
				}
				isDir = true
			} else {
				fi, err := os.Stat(path)
				if err != nil {
					return err
				}
				fileSize := fi.Size()
				size = &fileSize
			}

			nodes[path] = &FileNode{
				Path:     path,
				IsDir:    isDir,
				Size:     size,
				Children: make([]*FileNode, 0),
				Entities: filepathToEntities[path],
			}
			return nil
		},
	); err != nil {
		return ListFilesResponse{}, err
	}

	// Construct directory tree.
	var root *FileNode
	for path, node := range nodes {
		parentDir := filepath.Dir(path)
		parent, ok := nodes[parentDir]
		if ok {
			parent.AddChild(node)
		} else {
			root = node
		}
	}

	return ListFilesResponse{Root: root}, nil
}

type GetFileResponse struct {
	Content string `json:"content"`
}

// GetFileHandler returns the contents of the file at the requested location. Path is the absolute path to the file,
// irrespective of the dev server root.
func GetFileHandler(ctx context.Context, state *state.State, r *http.Request) (GetFileResponse, error) {
	path := r.URL.Query().Get("path")
	if path == "" {
		return GetFileResponse{}, libhttp.NewErrBadRequest("path is required")
	}

	// Ensure the path is within the dev server root.
	if !strings.HasPrefix(path, state.Dir) {
		return GetFileResponse{}, libhttp.NewErrBadRequest("path is outside dev root")
	}

	if strings.Contains(path, "..") {
		return GetFileResponse{}, libhttp.NewErrBadRequest("path may not contain directory traversal elements (`..`)")
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return GetFileResponse{}, errors.Wrap(err, "reading file")
	}

	return GetFileResponse{Content: string(contents)}, nil
}

type UpdateFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// UpdateFileHandler updates the file at the requested location. It takes in the new file contents in the request body,
// and updates the specified file only if the file path is within the development root.
func UpdateFileHandler(ctx context.Context, s *state.State, r *http.Request, req UpdateFileRequest) (struct{}, error) {
	if req.Path == "" {
		return struct{}{}, libhttp.NewErrBadRequest("path is required")
	}

	// Ensure the path is within the dev server root.
	if !strings.HasPrefix(req.Path, s.Dir) {
		return struct{}{}, libhttp.NewErrBadRequest("path is outside dev root")
	}

	if strings.Contains(req.Path, "..") {
		return struct{}{}, libhttp.NewErrBadRequest("path may not contain directory traversal elements (`..`)")
	}

	if err := os.WriteFile(req.Path, []byte(req.Content), 0644); err != nil {
		return struct{}{}, errors.Wrap(err, "writing file")
	}

	// TODO: check embedded requirements
	if s.SandboxState != nil {
		base := filepath.Base(req.Path)
		if base == "requirements.txt" || base == "package.json" || base == "yarn.lock" || base == "package-lock.json" {
			s.SandboxState.MarkDependenciesOutdated()
		}
	}

	return struct{}{}, nil
}

func DownloadBundleHandler(ctx context.Context, state *state.State, r *http.Request) ([]byte, string, error) {
	buf := new(bytes.Buffer)
	include, err := ignore.Func(state.Dir)
	if err != nil {
		return nil, "", errors.Wrap(err, "creating function to filter files")
	}

	if err := utils.Zip(buf, state.Dir, include); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), filepath.Base(state.Dir), nil
}

// ProxyViewHandler proxies requests to the Vite server for a view.
func ProxyViewHandler(portProxy *httputil.ReverseProxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		portProxy.ServeHTTP(w, r)
	}
}

type PatchRequest struct {
	Patches []string `json:"patches"`
}

// PatchHandler modifies a file using a unified diff patch.
func PatchHandler(ctx context.Context, s *state.State, r *http.Request, req PatchRequest) (struct{}, error) {
	if err := files.BatchPatch(s, req.Patches); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
}
