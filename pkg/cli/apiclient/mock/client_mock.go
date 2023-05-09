package mock

import (
	"context"

	"github.com/airplanedev/cli/pkg/cli/apiclient"
)

type MockClient struct {
	Tasks     map[string]api.Task
	Resources []api.Resource
	Views     map[string]api.View
}

var _ api.IAPIClient = &MockClient{}

func (mc *MockClient) GetTask(ctx context.Context, req api.GetTaskRequest) (res api.Task, err error) {
	task, ok := mc.Tasks[req.Slug]
	if !ok {
		return api.Task{}, &api.TaskMissingError{AppURL: "api/", Slug: req.Slug}
	}
	return task, nil
}

func (mc *MockClient) GetTaskMetadata(ctx context.Context, slug string) (res api.TaskMetadata, err error) {
	task, ok := mc.Tasks[slug]
	if !ok {
		return api.TaskMetadata{}, &api.TaskMissingError{AppURL: "api/", Slug: slug}
	}
	return api.TaskMetadata{
		ID:         task.ID,
		Slug:       task.Slug,
		IsArchived: task.IsArchived,
	}, nil
}

func (mc *MockClient) ListResources(ctx context.Context, envSlug string) (res api.ListResourcesResponse, err error) {
	return api.ListResourcesResponse{
		Resources: mc.Resources,
	}, nil
}

func (mc *MockClient) ListResourceMetadata(ctx context.Context) (res api.ListResourceMetadataResponse, err error) {
	metadata := []api.ResourceMetadata{}
	for i, r := range mc.Resources {
		metadata = append(metadata, api.ResourceMetadata{
			ID:                 r.ID,
			Slug:               r.Slug,
			DefaultEnvResource: &mc.Resources[i],
		})
	}
	return api.ListResourceMetadataResponse{
		Resources: metadata,
	}, nil
}

func (mc *MockClient) CreateBuildUpload(ctx context.Context, req api.CreateBuildUploadRequest) (res api.CreateBuildUploadResponse, err error) {
	return api.CreateBuildUploadResponse{
		WriteOnlyURL: "writeOnlyURL",
	}, nil
}

func (mc *MockClient) GetView(ctx context.Context, req api.GetViewRequest) (res api.View, err error) {
	a, ok := mc.Views[req.Slug]
	if !ok {
		return api.View{}, &api.ViewMissingError{AppURL: "api/", Slug: req.Slug}
	}
	return a, nil
}
