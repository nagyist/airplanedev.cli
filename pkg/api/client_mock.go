package api

import (
	"context"
	"fmt"
	"time"

	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/pkg/errors"
)

type MockClient struct {
	Tasks                 map[string]libapi.Task
	Views                 map[string]libapi.View
	Deploys               []CreateDeploymentRequest
	GetDeploymentResponse *Deployment
	Resources             []libapi.Resource
	Configs               []Config
	Envs                  map[string]Env
}

var _ APIClient = &MockClient{}

func (mc *MockClient) GetTask(ctx context.Context, req libapi.GetTaskRequest) (res libapi.Task, err error) {
	task, ok := mc.Tasks[req.Slug]
	if !ok {
		return libapi.Task{}, &libapi.TaskMissingError{AppURL: "api/", Slug: req.Slug}
	}
	return task, nil
}

func (mc *MockClient) GetTaskMetadata(ctx context.Context, slug string) (res libapi.TaskMetadata, err error) {
	task, ok := mc.Tasks[slug]
	if !ok {
		return libapi.TaskMetadata{}, &libapi.TaskMissingError{AppURL: "api/", Slug: slug}
	}
	return libapi.TaskMetadata{
		ID:   task.ID,
		Slug: task.Slug,
	}, nil
}

func (mc *MockClient) ListTasks(ctx context.Context, envSlug string) (res ListTasksResponse, err error) {
	panic("not implemented") // TODO: Implement
}

func (mc *MockClient) RunTask(ctx context.Context, req RunTaskRequest) (RunTaskResponse, error) {
	panic("not implemented")
}

func (mc *MockClient) GetRun(ctx context.Context, id string) (res GetRunResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) GetOutputs(ctx context.Context, runID string) (res GetOutputsResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) ListResources(ctx context.Context, envSlug string) (res libapi.ListResourcesResponse, err error) {
	return libapi.ListResourcesResponse{Resources: mc.Resources}, nil
}

func (mc *MockClient) ListResourceMetadata(ctx context.Context) (res libapi.ListResourceMetadataResponse, err error) {
	metadata := []libapi.ResourceMetadata{}
	for i, r := range mc.Resources {
		metadata = append(metadata, libapi.ResourceMetadata{
			ID:                 r.ID,
			Slug:               r.Slug,
			DefaultEnvResource: &mc.Resources[i],
		})
	}
	return libapi.ListResourceMetadataResponse{
		Resources: metadata,
	}, nil
}

func (mc *MockClient) SetConfig(ctx context.Context, req SetConfigRequest) (err error) {
	panic("not implemented") // TODO: Implement
}

func (mc *MockClient) GetConfig(ctx context.Context, req GetConfigRequest) (res GetConfigResponse, err error) {
	for _, c := range mc.Configs {
		if c.Name == req.Name && c.Tag == req.Tag {
			return GetConfigResponse{Config: c}, nil
		}
	}

	return GetConfigResponse{}, errors.Errorf("config %s does not exist", req.Name)
}

func (mc *MockClient) TaskURL(slug string, envSlug string) string {
	if envSlug != "" {
		return fmt.Sprintf("api/t/%s?__env=%s", slug, envSlug)
	}
	return fmt.Sprintf("api/t/%s", slug)
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
	task.Runtime = req.Runtime
	task.Repo = req.Repo
	if req.RequireExplicitPermissions != nil {
		task.RequireExplicitPermissions = *req.RequireExplicitPermissions
	}
	if req.Permissions != nil {
		task.Permissions = *req.Permissions
	}
	task.Timeout = req.Timeout
	if req.InterpolationMode != nil {
		task.InterpolationMode = *req.InterpolationMode
	}
	mc.Tasks[req.Slug] = task

	return UpdateTaskResponse{}, nil
}

func (mc *MockClient) CreateTask(ctx context.Context, req CreateTaskRequest) (res CreateTaskResponse, err error) {
	panic("not implemented") // TODO: Implement
}

// TODO add other functions when needed.
func (mc *MockClient) GetRegistryToken(ctx context.Context) (res RegistryTokenResponse, err error) {
	return RegistryTokenResponse{Token: "token"}, nil
}

func (mc *MockClient) CreateBuildUpload(ctx context.Context, req libapi.CreateBuildUploadRequest) (res libapi.CreateBuildUploadResponse, err error) {
	return libapi.CreateBuildUploadResponse{
		WriteOnlyURL: "writeOnlyURL",
	}, nil
}

func (mc *MockClient) GetDeploymentLogs(ctx context.Context, id string, prevToken string) (res GetDeploymentLogsResponse, err error) {
	return GetDeploymentLogsResponse{}, nil
}

func (mc *MockClient) GetDeployment(ctx context.Context, id string) (res Deployment, err error) {
	if mc.GetDeploymentResponse != nil {
		return *mc.GetDeploymentResponse, nil
	}
	return Deployment{
		SucceededAt: &time.Time{},
	}, nil
}

func (mc *MockClient) CreateDeployment(ctx context.Context, req CreateDeploymentRequest) (res CreateDeploymentResponse, err error) {
	mc.Deploys = append(mc.Deploys, req)
	return CreateDeploymentResponse{
		Deployment: Deployment{
			ID: "deployment",
		},
		NumTasksUpdated:  len(req.Tasks),
		NumBuildsCreated: len(req.Tasks),
	}, nil
}

func (mc *MockClient) CancelDeployment(ctx context.Context, req CancelDeploymentRequest) error {
	return nil
}

// DeploymentURL returns a URL for a deployment.
func (mc *MockClient) DeploymentURL(deploymentID string, envSlug string) string {
	if envSlug != "" {
		return fmt.Sprintf("https://airplane.dev/%s?__env=%s", deploymentID, envSlug)
	}
	return fmt.Sprintf("https://airplane.dev/%s", deploymentID)
}

func (mc *MockClient) GetView(ctx context.Context, req libapi.GetViewRequest) (res libapi.View, err error) {
	a, ok := mc.Views[req.Slug]
	if !ok {
		return libapi.View{}, &libapi.ViewMissingError{AppURL: "api/", Slug: req.Slug}
	}
	return a, nil
}

func (mc *MockClient) CreateView(ctx context.Context, req libapi.CreateViewRequest) (res libapi.View, err error) {
	panic("not implemented") // TODO: Implement
}

func (mc *MockClient) CreateDemoDB(ctx context.Context, name string) (string, error) {
	panic("not implemented")
}

func (mc *MockClient) GetEnv(ctx context.Context, envSlug string) (Env, error) {
	env, ok := mc.Envs[envSlug]
	if !ok {
		return Env{}, errors.Errorf("environment with slug %s does not exist", envSlug)
	}
	return env, nil
}

func (mc *MockClient) GetResource(ctx context.Context, req GetResourceRequest) (res libapi.GetResourceResponse, err error) {
	for _, r := range mc.Resources {
		if r.Slug == req.Slug {
			return libapi.GetResourceResponse{Resource: r}, nil
		}
	}

	return libapi.GetResourceResponse{}, errors.Errorf("resource with slug %s does not exist", req.Slug)
}

func (mc *MockClient) GetToken() string {
	return "mock-token"
}
