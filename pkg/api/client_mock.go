package api

import (
	"context"

	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/pkg/errors"
)

type MockClient struct {
	Tasks map[string]libapi.Task
}

var _ APIClient = &MockClient{}

func (mc *MockClient) GetTask(ctx context.Context, req libapi.GetTaskRequest) (res libapi.Task, err error) {
	task, ok := mc.Tasks[req.Slug]
	if !ok {
		return libapi.Task{}, &libapi.TaskMissingError{AppURL: "api/", Slug: req.Slug}
	}
	return task, nil
}

func (mc *MockClient) ListTasks(ctx context.Context, envSlug string) (res ListTasksResponse, err error) {
	panic("not implemented") // TODO: Implement
}

func (mc *MockClient) ListResources(ctx context.Context) (res libapi.ListResourcesResponse, err error) {
	return libapi.ListResourcesResponse{}, nil
}
func (mc *MockClient) SetConfig(ctx context.Context, req SetConfigRequest) (err error) {
	panic("not implemented") // TODO: Implement
}

func (mc *MockClient) GetConfig(ctx context.Context, req GetConfigRequest) (res GetConfigResponse, err error) {
	panic("not implemented") // TODO: Implement
}

func (mc *MockClient) TaskURL(slug string) string {
	return "api/t/" + slug
}

func (mc *MockClient) UpdateTask(ctx context.Context, req libapi.UpdateTaskRequest) (res UpdateTaskResponse, err error) {
	task, ok := mc.Tasks[req.Slug]
	if !ok {
		return UpdateTaskResponse{}, errors.Errorf("no task %s", req.Slug)
	}
	task.Name = req.Name
	task.Arguments = req.Arguments
	task.Command = req.Command
	task.Description = req.Description
	task.Image = req.Image
	task.Parameters = req.Parameters
	task.Constraints = req.Constraints
	task.Env = req.Env
	task.ResourceRequests = req.ResourceRequests
	task.Resources = req.Resources
	task.Kind = req.Kind
	task.KindOptions = req.KindOptions
	task.Repo = req.Repo
	task.RequireExplicitPermissions = req.RequireExplicitPermissions
	task.Permissions = req.Permissions
	task.Timeout = req.Timeout
	task.InterpolationMode = req.InterpolationMode
	mc.Tasks[req.Slug] = task

	return UpdateTaskResponse{}, nil
}

func (mc *MockClient) CreateTask(ctx context.Context, req CreateTaskRequest) (res CreateTaskResponse, err error) {
	panic("not implemented") // TODO: Implement
}

// TODO add other functions when needed.
func (mc *MockClient) CreateBuild(ctx context.Context, req CreateBuildRequest) (res CreateBuildResponse, err error) {
	panic("not implemented") // TODO: Implement
}

func (mc *MockClient) GetRegistryToken(ctx context.Context) (res RegistryTokenResponse, err error) {
	return RegistryTokenResponse{Token: "token"}, nil
}

func (mc *MockClient) CreateBuildUpload(ctx context.Context, req libapi.CreateBuildUploadRequest) (res libapi.CreateBuildUploadResponse, err error) {
	return libapi.CreateBuildUploadResponse{
		WriteOnlyURL: "writeOnlyURL",
	}, nil
}

func (mc *MockClient) GetBuildLogs(ctx context.Context, buildID string, prevToken string) (res GetBuildLogsResponse, err error) {
	panic("not implemented") // TODO: Implement
}

func (mc *MockClient) GetBuild(ctx context.Context, id string) (res GetBuildResponse, err error) {
	panic("not implemented") // TODO: Implement
}

func (mc *MockClient) ListGroups(ctx context.Context) (res libapi.ListGroupsResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) ListUsers(ctx context.Context) (res libapi.ListUsersResponse, err error) {
	panic("not implemented")
}
