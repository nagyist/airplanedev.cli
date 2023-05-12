package tasks

import (
	"context"
	"net/http"
	"time"

	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/cli/apiclient/http"
	resources "github.com/airplanedev/cli/pkg/cli/resources/cliresources"
	"github.com/airplanedev/cli/pkg/cli/server/state"
	serverutils "github.com/airplanedev/cli/pkg/cli/server/utils"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"golang.org/x/exp/slices"
)

type GetTaskResponse struct {
	libapi.Task

	// File is the (absolute) path to the file where this task is defined.
	File string `json:"file"`
}

// GetTaskHandler handles requests to the /i/tasks/get?slug=<task_slug> endpoint.
func GetTaskHandler(ctx context.Context, state *state.State, r *http.Request) (GetTaskResponse, error) {
	taskSlug := r.URL.Query().Get("slug")
	if taskSlug == "" {
		return GetTaskResponse{}, libhttp.NewErrBadRequest("task slug was not supplied")
	}

	taskState, ok := state.LocalTasks.Get(taskSlug)
	if !ok {
		return GetTaskResponse{}, libhttp.NewErrNotFound("task with slug %q not found", taskSlug)
	}

	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	task, err := taskStateToAPITask(ctx, state, taskState, envSlug)
	if err != nil {
		return GetTaskResponse{}, err
	}

	return GetTaskResponse{
		Task: task,
		File: taskState.Def.GetDefnFilePath(),
	}, nil
}

type UpdateTaskRequest struct {
	libapi.UpdateTaskRequest

	// With Studio, schedules are edited in-tandem with the task. Accept both in the same endpoint.
	Triggers []libapi.Trigger `json:"triggers"`
}

func UpdateTaskHandler(ctx context.Context, s *state.State, r *http.Request, req UpdateTaskRequest) (struct{}, error) {
	taskConfig, ok := s.LocalTasks.Get(req.Slug)
	if !ok {
		return struct{}{}, libhttp.NewErrNotFound("task with slug %q not found", req.Slug)
	}

	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(s, r)
	resourceMetadata, err := resources.ListResourceMetadata(ctx, s.RemoteClient, s.DevConfig, envSlug)
	if err != nil {
		return struct{}{}, err
	}

	var users []libapi.User
	var groups []libapi.Group
	if pointers.ToBool(req.UpdateTaskRequest.RequireExplicitPermissions) {
		usersResp, err := s.RemoteClient.ListTeamUsers(ctx, s.AuthInfo.Team.ID)
		if err != nil {
			return struct{}{}, err
		}

		for _, user := range usersResp.Users {
			users = append(users, user.User)
		}

		groupsResp, err := s.RemoteClient.ListGroups(ctx)
		if err != nil {
			return struct{}{}, err
		}
		groups = groupsResp.Groups
	}

	if err := taskConfig.Def.Update(req.UpdateTaskRequest, definitions.UpdateOptions{
		Triggers:           req.Triggers,
		AvailableResources: resourceMetadata,
		Users:              users,
		Groups:             groups,
	}); err != nil {
		return struct{}{}, libhttp.NewErrBadRequest("unable to update task %q: %s", req.Slug, err.Error())
	}

	kind, err := taskConfig.Def.Kind()
	if err != nil {
		return struct{}{}, err
	}

	rt, err := runtime.Lookup(taskConfig.TaskEntrypoint, kind)
	if err != nil {
		return struct{}{}, err
	}

	// Update the underlying task file.
	if err := rt.Update(ctx, s.Logger, taskConfig.Def.GetDefnFilePath(), req.Slug, taskConfig.Def); err != nil {
		return struct{}{}, err
	}

	// Optimistically update the task in the cache.
	_, err = s.LocalTasks.Update(req.Slug, func(val *state.TaskState) error {
		val.Def = taskConfig.Def
		val.UpdatedAt = time.Now()
		return nil
	})
	if err != nil {
		return struct{}{}, err
	}

	return struct{}{}, nil
}

type CanUpdateTaskRequest struct {
	Slug string `json:"slug"`
}

type CanUpdateTaskResponse struct {
	CanUpdate bool `json:"canUpdate"`
}

func CanUpdateTaskHandler(ctx context.Context, state *state.State, r *http.Request) (CanUpdateTaskResponse, error) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		return CanUpdateTaskResponse{}, libhttp.NewErrBadRequest("task slug was not supplied")
	}

	taskConfig, ok := state.LocalTasks.Get(slug)
	if !ok {
		return CanUpdateTaskResponse{}, libhttp.NewErrNotFound("task with slug %q not found", slug)
	}

	kind, err := taskConfig.Def.Kind()
	if err != nil {
		return CanUpdateTaskResponse{}, err
	}

	rt, err := runtime.Lookup(taskConfig.TaskEntrypoint, kind)
	if err != nil {
		return CanUpdateTaskResponse{}, err
	}

	canUpdate, err := rt.CanUpdate(ctx, state.Logger, taskConfig.Def.GetDefnFilePath(), slug)
	if err != nil {
		return CanUpdateTaskResponse{}, err
	}

	return CanUpdateTaskResponse{
		CanUpdate: canUpdate,
	}, nil
}

func ListTasksHandler(ctx context.Context, state *state.State, r *http.Request) (api.ListTasksResponse, error) {
	tasks, err := ListTasks(ctx, state)
	if err != nil {
		return api.ListTasksResponse{}, err
	}

	return api.ListTasksResponse{
		Tasks: tasks,
	}, nil
}

func ListTasks(ctx context.Context, s *state.State) ([]libapi.Task, error) {
	taskStates := s.LocalTasks.Values()
	tasks := make([]libapi.Task, 0, len(taskStates))

	for _, taskState := range taskStates {
		t, err := taskStateToAPITask(ctx, s, taskState, nil)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}

	slices.SortFunc(tasks, func(a, b libapi.Task) bool {
		return a.Slug < b.Slug
	})

	return tasks, nil
}

func taskStateToAPITask(
	ctx context.Context,
	state *state.State,
	taskState state.TaskState,
	envSlug *string,
) (libapi.Task, error) {
	var resourceMetadata []libapi.ResourceMetadata
	resourceAttachments, err := taskState.Def.GetResourceAttachments()
	if err != nil {
		return libapi.Task{}, err
	}
	if len(resourceAttachments) > 0 {
		var err error
		resourceMetadata, err = resources.ListResourceMetadata(ctx, state.RemoteClient, state.DevConfig, envSlug)
		if err != nil {
			return libapi.Task{}, err
		}
	}

	var users []libapi.User
	var groups []libapi.Group
	if taskState.Def.Permissions != nil && taskState.Def.Permissions.RequireExplicitPermissions {
		usersResp, err := state.RemoteClient.ListTeamUsers(ctx, state.AuthInfo.Team.ID)
		if err != nil {
			return libapi.Task{}, err
		}

		for _, user := range usersResp.Users {
			users = append(users, user.User)
		}

		groupsResp, err := state.RemoteClient.ListGroups(ctx)
		if err != nil {
			return libapi.Task{}, err
		}
		groups = groupsResp.Groups
	}

	task, err := taskState.Def.GetTask(definitions.GetTaskOpts{
		AvailableResources: resourceMetadata,
		Users:              users,
		Groups:             groups,
		Bundle:             true,
		// We want to best-effort support invalid task definitions (e.g. unknown resources) so that
		// we can render corresponding validation errors in the UI.
		IgnoreInvalid: true,
	})
	if err != nil {
		return libapi.Task{}, libhttp.NewErrBadRequest("task %q is invalid: %s", taskState.TaskID, err.Error())
	}

	// Use the studio-generated ID.
	task.ID = taskState.TaskID
	task.UpdatedAt = taskState.UpdatedAt

	// Certain fields are not supported by tasks-as-code, so give them default values.
	if task.ResourceRequests == nil {
		task.ResourceRequests = libapi.ResourceRequests{}
	}
	if task.InterpolationMode == "" {
		task.InterpolationMode = "jst"
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = task.UpdatedAt
	}

	return task, nil
}
