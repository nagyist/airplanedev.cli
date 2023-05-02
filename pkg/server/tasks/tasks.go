package tasks

import (
	"context"
	"net/http"

	libapi "github.com/airplanedev/cli/pkg/api"
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/api/http"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	resources "github.com/airplanedev/cli/pkg/resources/cliresources"
	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/server/state"
	serverutils "github.com/airplanedev/cli/pkg/server/utils"
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

	taskConfig, ok := state.TaskConfigs.Get(taskSlug)
	if !ok {
		return GetTaskResponse{}, libhttp.NewErrNotFound("task with slug %q not found", taskSlug)
	}

	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	task, err := taskConfigToAPITask(ctx, state, taskConfig, envSlug)
	if err != nil {
		return GetTaskResponse{}, err
	}

	return GetTaskResponse{
		Task: task,
		File: taskConfig.Def.GetDefnFilePath(),
	}, nil
}

type UpdateTaskRequest struct {
	libapi.UpdateTaskRequest

	// With Studio, schedules are edited in-tandem with the task. Accept both in the same endpoint.
	Triggers []libapi.Trigger `json:"triggers"`
}

func UpdateTaskHandler(ctx context.Context, state *state.State, r *http.Request, req UpdateTaskRequest) (struct{}, error) {
	taskConfig, ok := state.TaskConfigs.Get(req.Slug)
	if !ok {
		return struct{}{}, libhttp.NewErrNotFound("task with slug %q not found", req.Slug)
	}

	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	resources, err := resources.ListResourceMetadata(ctx, state.RemoteClient, state.DevConfig, envSlug)
	if err != nil {
		return struct{}{}, err
	}

	if err := taskConfig.Def.Update(req.UpdateTaskRequest, definitions.UpdateOptions{
		Triggers:           req.Triggers,
		AvailableResources: resources,
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
	if err := rt.Update(ctx, state.Logger, taskConfig.Def.GetDefnFilePath(), req.Slug, taskConfig.Def); err != nil {
		return struct{}{}, err
	}

	// Optimistically update the task in the cache.
	_, err = state.TaskConfigs.Update(req.Slug, func(val *discover.TaskConfig) error {
		val.Def = taskConfig.Def
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

	taskConfig, ok := state.TaskConfigs.Get(slug)
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
	taskConfigs := s.TaskConfigs.Values()
	tasks := make([]libapi.Task, 0, len(taskConfigs))

	for _, taskConfig := range taskConfigs {
		t, err := taskConfigToAPITask(ctx, s, taskConfig, nil)
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

func taskConfigToAPITask(
	ctx context.Context,
	state *state.State,
	taskConfig discover.TaskConfig,
	envSlug *string,
) (libapi.Task, error) {
	resources, err := resources.ListResourceMetadata(ctx, state.RemoteClient, state.DevConfig, envSlug)
	if err != nil {
		return libapi.Task{}, err
	}

	task, err := taskConfig.Def.GetTask(definitions.GetTaskOpts{
		AvailableResources: resources,
		Bundle:             true,
		// We want to best-effort support invalid task definitions (e.g. unknown resources) so that
		// we can render corresponding validation errors in the UI.
		IgnoreInvalid: true,
	})
	if err != nil {
		return libapi.Task{}, libhttp.NewErrBadRequest("task %q is invalid: %s", taskConfig.TaskID, err.Error())
	}

	// Use the studio-generated ID.
	task.ID = taskConfig.TaskID

	metadata, err := state.GetTaskErrors(ctx, task.Slug, pointers.ToString(envSlug))
	if err != nil {
		return libapi.Task{}, err
	}
	task.UpdatedAt = metadata.RefreshedAt

	// Certain fields are not supported by tasks-as-code, so give them default values.
	if task.ResourceRequests == nil {
		task.ResourceRequests = libapi.ResourceRequests{}
	}
	if task.Permissions == nil {
		task.Permissions = libapi.Permissions{}
	}
	if task.InterpolationMode == "" {
		task.InterpolationMode = "jst"
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = task.UpdatedAt
	}

	return task, nil
}
