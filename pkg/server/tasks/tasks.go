package tasks

import (
	"context"
	"net/http"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/resources"
	"github.com/airplanedev/cli/pkg/server/state"
	serverutils "github.com/airplanedev/cli/pkg/server/utils"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	libapi "github.com/airplanedev/lib/pkg/api"
	libhttp "github.com/airplanedev/lib/pkg/api/http"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/runtime"
)

// GetTaskHandler handles requests to the /i/tasks/get?slug=<task_slug> endpoint.
func GetTaskHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.Task, error) {
	taskSlug := r.URL.Query().Get("slug")
	if taskSlug == "" {
		return libapi.Task{}, libhttp.NewErrBadRequest("task slug was not supplied")
	}

	taskConfig, ok := state.TaskConfigs.Get(taskSlug)
	if !ok {
		return libapi.Task{}, libhttp.NewErrNotFound("task with slug %q not found", taskSlug)
	}

	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	task, err := taskConfigToAPITask(ctx, state, taskConfig, envSlug)
	if err != nil {
		return libapi.Task{}, err
	}

	return task, nil
}

func UpdateTaskHandler(ctx context.Context, state *state.State, r *http.Request, req libapi.UpdateTaskRequest) (struct{}, error) {
	taskConfig, ok := state.TaskConfigs.Get(req.Slug)
	if !ok {
		return struct{}{}, libhttp.NewErrNotFound("task with slug %q not found", req.Slug)
	}

	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	resources, err := resources.ListResourceMetadata(ctx, state.RemoteClient, state.DevConfig, envSlug)
	if err != nil {
		return struct{}{}, err
	}

	if err := taskConfig.Def.Update(req, definitions.UpdateOptions{
		// TODO(colin, 04012023): add support for updating schedules. For now, we leave them as-is.
		Triggers:           nil,
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
	if err := rt.Edit(ctx, state.Logger, taskConfig.TaskEntrypoint, req.Slug, taskConfig.Def); err != nil {
		return struct{}{}, err
	}

	return struct{}{}, nil
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
	if task.Triggers == nil {
		task.Triggers = []libapi.Trigger{}
	}
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
