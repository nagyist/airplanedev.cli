package apidev

import (
	"context"
	"net/http"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	libhttp "github.com/airplanedev/cli/pkg/api/http"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/cli/pkg/initcmd"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
)

const (
	postInitPollMaximum  = time.Second * 5
	postInitPollInterval = time.Millisecond * 10
)

type InitTaskRequest struct {
	DryRun bool `json:"dryRun"`

	Name        string              `json:"name"`
	Description string              `json:"description"`
	Kind        buildtypes.TaskKind `json:"kind"`
	KindName    string              `json:"kindName"`
	NodeFlavor  initcmd.NodeFlavor  `json:"nodeFlavor"`
}

type InitTaskResponse struct {
	Task         *api.Task                    `json:"task"`
	UpdatedFiles []initcmd.FilenameWithStatus `json:"updatedFiles"`
}

func InitTaskHandler(ctx context.Context, s *state.State, r *http.Request, req InitTaskRequest) (InitTaskResponse, error) {
	if req.Name == "" {
		return InitTaskResponse{}, libhttp.NewErrBadRequest("missing name")
	}
	if req.Kind == "" {
		return InitTaskResponse{}, libhttp.NewErrBadRequest("missing kind")
	}

	resp, err := initcmd.InitTask(ctx, initcmd.InitTaskRequest{
		Client:           s.RemoteClient,
		DryRun:           req.DryRun,
		WorkingDirectory: s.Dir,
		Inline:           true,
		TaskName:         req.Name,
		TaskKind:         req.Kind,
		TaskKindName:     req.KindName,
		TaskDescription:  req.Description,
		TaskNodeFlavor:   req.NodeFlavor,
	})
	if err != nil {
		return InitTaskResponse{}, errors.Wrap(err, "initializing task")
	}

	var newTask *api.Task
	if !req.DryRun && resp.NewTaskDefinition != nil {
		t, err := resp.NewTaskDefinition.GetTask(definitions.GetTaskOpts{
			Bundle:        true,
			IgnoreInvalid: true,
		})
		if err != nil {
			return InitTaskResponse{}, errors.Wrap(err, "converting definition to task")
		}
		newTask = &t

		// Kick off discovery for the task.
		path := resp.NewTaskDefinition.GetDefnFilePath()
		if path == "" {
			entrypoint, err := resp.NewTaskDefinition.GetAbsoluteEntrypoint()
			if err != nil {
				return InitTaskResponse{}, errors.Wrap(err, "getting absolute entrypoint")
			}
			path = entrypoint
		}
		if err := s.ReloadPath(ctx, path); err != nil {
			return InitTaskResponse{}, errors.Wrap(err, "reloading path")
		}

		// Wait a maximum of five seconds for the task to be discovered.
		utils.WaitUntilTimeout(func() bool {
			if _, ok := s.TaskConfigs.Get(t.Slug); ok {
				return true
			}
			return false
		}, postInitPollInterval, postInitPollMaximum)
	}

	return InitTaskResponse{
		UpdatedFiles: resp.GetFilenamesWithStatus(),
		Task:         newTask,
	}, nil
}

type InitViewRequest struct {
	DryRun bool `json:"dryRun"`

	Name        string `json:"name"`
	Description string `json:"description"`
}

type InitViewResponse struct {
	Slug         *string                      `json:"slug,omitempty"`
	UpdatedFiles []initcmd.FilenameWithStatus `json:"updatedFiles"`
}

func InitViewHandler(ctx context.Context, s *state.State, r *http.Request, req InitViewRequest) (InitViewResponse, error) {
	if req.Name == "" {
		return InitViewResponse{}, libhttp.NewErrBadRequest("missing name")
	}

	resp, err := initcmd.InitView(ctx, initcmd.InitViewRequest{
		DryRun:           req.DryRun,
		WorkingDirectory: s.Dir,
		Name:             req.Name,
		Description:      req.Description,
	})
	if err != nil {
		return InitViewResponse{}, errors.Wrap(err, "initializing view")
	}

	if !req.DryRun && resp.NewViewDefinition != nil {
		// Kick off discovery for the view.
		if err := s.ReloadPath(ctx, resp.NewViewDefinition.DefnFilePath); err != nil {
			return InitViewResponse{}, errors.Wrap(err, "reloading path")
		}

		// Wait a maximum of five seconds for the task to be discovered.
		utils.WaitUntilTimeout(func() bool {
			if _, ok := s.ViewConfigs.Get(resp.NewViewDefinition.Slug); ok {
				return true
			}
			return false
		}, postInitPollInterval, postInitPollMaximum)
	}

	return InitViewResponse{
		UpdatedFiles: resp.GetFilenamesWithStatus(),
		Slug:         &resp.NewViewDefinition.Slug,
	}, nil
}
