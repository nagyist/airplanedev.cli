package api

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/pkg/errors"
)

type MockClient struct {
	token                 string
	Configs               []Config
	Deploys               []CreateDeploymentRequest
	Envs                  map[string]libapi.Env
	GetDeploymentResponse *Deployment
	Resources             []libapi.Resource
	Runbooks              map[string]Runbook
	SessionBlocks         map[string][]SessionBlock
	Tasks                 map[string]libapi.Task
	Users                 map[string]User
	Views                 map[string]libapi.View
	Uploads               map[string]libapi.Upload
	apiKey                string
	source                string
	teamID                string
	tunnelToken           *string
}

var _ APIClient = &MockClient{}

func NewMockClient() *MockClient {
	return &MockClient{
		token: "mock-token",
	}
}

func (mc *MockClient) AuthInfo(ctx context.Context) (res AuthInfoResponse, err error) {
	return AuthInfoResponse{}, nil
}

func (mc *MockClient) GetTask(ctx context.Context, req libapi.GetTaskRequest) (res libapi.Task, err error) {
	task, ok := mc.Tasks[req.Slug]
	if !ok {
		return libapi.Task{}, &libapi.TaskMissingError{AppURL: "api/", Slug: req.Slug}
	}
	return task, nil
}

func (mc *MockClient) GetTaskByID(ctx context.Context, id string) (res libapi.Task, err error) {
	task, ok := mc.Tasks[id]
	if !ok {
		return libapi.Task{}, errors.New("no task found")
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

func (mc *MockClient) GetTaskReviewers(ctx context.Context, slug string) (res GetTaskReviewersResponse, err error) {
	task, ok := mc.Tasks[slug]
	if !ok {
		return GetTaskReviewersResponse{}, &libapi.TaskMissingError{AppURL: "api/", Slug: slug}
	}
	return GetTaskReviewersResponse{
		Task: &task,
	}, nil
}

func (mc *MockClient) ListTasks(ctx context.Context, envSlug string) (res ListTasksResponse, err error) {
	allTasks := make([]libapi.Task, 0, len(mc.Tasks))
	for _, task := range mc.Tasks {
		allTasks = append(allTasks, task)
	}
	return ListTasksResponse{
		Tasks: allTasks,
	}, nil
}

func (mc *MockClient) RunTask(ctx context.Context, req RunTaskRequest) (RunTaskResponse, error) {
	return RunTaskResponse{RunID: utils.GenerateID("run")}, nil
}

func (mc *MockClient) GetRun(ctx context.Context, id string) (res GetRunResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) GetOutputs(ctx context.Context, runID string) (res GetOutputsResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) GetRunbook(ctx context.Context, runbookSlug string, envSlug string) (res GetRunbookResponse, err error) {
	runbook, ok := mc.Runbooks[runbookSlug]
	if !ok {
		return GetRunbookResponse{}, errors.New("runbook not found")
	}
	return GetRunbookResponse{
		Runbook: runbook,
	}, nil
}

func (mc *MockClient) ListSessionBlocks(ctx context.Context, sessionID string) (res ListSessionBlocksResponse, err error) {
	blocks, ok := mc.SessionBlocks[sessionID]
	if !ok {
		return ListSessionBlocksResponse{}, errors.New("blocks not found")
	}
	return ListSessionBlocksResponse{
		Blocks: blocks,
	}, nil
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

func (mc *MockClient) ListConfigs(ctx context.Context, req ListConfigsRequest) (res ListConfigsResponse, err error) {
	return ListConfigsResponse{Configs: mc.Configs}, nil
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

func (mc *MockClient) ListRuns(ctx context.Context, req ListRunsRequest) (ListRunsResponse, error) {
	panic("not implemented")
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

func (mc *MockClient) ResetDemoDB(ctx context.Context) (string, error) {
	panic("not implemented")
}

func (mc *MockClient) ListFlags(ctx context.Context) (res ListFlagsResponse, err error) {
	panic("not implemented") // TODO: Implement
}

func (mc *MockClient) GetEnv(ctx context.Context, envSlug string) (libapi.Env, error) {
	env, ok := mc.Envs[envSlug]
	if !ok {
		return libapi.Env{}, errors.Errorf("environment with slug %s does not exist", envSlug)
	}
	return env, nil
}

func (mc *MockClient) ListEnvs(ctx context.Context) (ListEnvsResponse, error) {
	envs := []libapi.Env{}
	for _, env := range mc.Envs {
		envs = append(envs, env)
	}
	return ListEnvsResponse{
		Envs: envs,
	}, nil
}

func (mc *MockClient) GetResource(ctx context.Context, req GetResourceRequest) (res libapi.GetResourceResponse, err error) {
	for _, r := range mc.Resources {
		if r.Slug != "" && r.Slug == req.Slug {
			return libapi.GetResourceResponse{Resource: r}, nil
		} else if r.ID != "" && r.ID == req.ID {
			return libapi.GetResourceResponse{Resource: r}, nil
		}
	}

	return libapi.GetResourceResponse{}, errors.Errorf("resource with slug %s does not exist", req.Slug)
}

func (mc *MockClient) Token() string {
	return mc.token
}

func (mc *MockClient) SetToken(token string) {
	mc.token = token
}

func (mc *MockClient) SetHost(host string) {
}

func (mc *MockClient) TunnelToken() *string {
	return mc.tunnelToken
}

func (mc *MockClient) APIKey() string {
	return mc.apiKey
}

func (mc *MockClient) SetAPIKey(apiKey string) {
	mc.apiKey = apiKey
}

func (mc *MockClient) TeamID() string {
	return mc.teamID
}

func (mc *MockClient) SetTeamID(teamID string) {
	mc.teamID = teamID
}

func (mc *MockClient) Source() string {
	return mc.source
}

func (mc *MockClient) SetSource(source string) {
	mc.source = source
}

func (mc *MockClient) AppURL() *url.URL {
	panic("not implemented")
}

func (mc *MockClient) EvaluateTemplate(ctx context.Context, req libapi.EvaluateTemplateRequest) (res libapi.EvaluateTemplateResponse, err error) {
	switch requestVal := req.Value.(type) {
	case map[string]string: // Just convert map[string]string to map[string]interface{}, which is what our prod API returns.
		value := make(map[string]interface{}, len(requestVal))
		for k, v := range requestVal {
			value[k] = v
		}
		return libapi.EvaluateTemplateResponse{Value: value}, nil
	}

	return libapi.EvaluateTemplateResponse{
		Value: req.Value,
	}, nil
}

func (mc *MockClient) GetPermissions(ctx context.Context, taskSlug string, actions []string) (res GetPermissionsResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) GenerateSignedURLs(ctx context.Context, envSlug string) (res GenerateSignedURLsResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) GetWebHost(ctx context.Context) (res string, err error) {
	panic("not implemented")
}

func (mc *MockClient) GetUser(ctx context.Context, userID string) (res GetUserResponse, err error) {
	if user, ok := mc.Users[userID]; !ok {
		return GetUserResponse{}, errors.Errorf("user with id %s does not exist", userID)
	} else {
		return GetUserResponse{User: user}, nil
	}
}

func (mc *MockClient) CreateUpload(ctx context.Context, req libapi.CreateUploadRequest) (res libapi.CreateUploadResponse, err error) {
	id := utils.GenerateID("upl")
	upload := libapi.Upload{
		ID:        id,
		FileName:  req.FileName,
		SizeBytes: req.SizeBytes,
	}
	mc.Uploads[id] = upload

	return libapi.CreateUploadResponse{
		Upload: upload,
	}, nil
}

func (mc *MockClient) GetUpload(ctx context.Context, uploadID string) (res libapi.GetUploadResponse, err error) {
	if upload, ok := mc.Uploads[uploadID]; !ok {
		return libapi.GetUploadResponse{}, errors.Errorf("upload with id %s does not exist", uploadID)
	} else {
		return libapi.GetUploadResponse{
			Upload:      upload,
			ReadOnlyURL: "fake-url",
		}, nil
	}
}

func (mc *MockClient) GetTunnelToken(ctx context.Context) (res GetTunnelTokenResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) CreateSandbox(ctx context.Context, req CreateSandboxRequest) (res CreateSandboxResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) SetDevSecret(ctx context.Context, token string) (err error) {
	panic("not implemented")
}

func (mc *MockClient) CreateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (res CreateAPIKeyResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) ListAPIKeys(ctx context.Context) (res ListAPIKeysResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) DeleteAPIKey(ctx context.Context, req DeleteAPIKeyRequest) (err error) {
	panic("not implemented")
}

func (mc *MockClient) Host() string {
	return ""
}

func (mc *MockClient) GetUniqueSlug(ctx context.Context, name, preferredSlug string) (res GetUniqueSlugResponse, err error) {
	panic("not implemented")
}

func (mc *MockClient) TokenURL() string {
	panic("not implemented")
}

func (mc *MockClient) LoginURL(uri string) string {
	panic("not implemented")
}

func (mc *MockClient) LoginSuccessURL() string {
	panic("not implemented")
}

func (mc *MockClient) Watcher(ctx context.Context, req RunTaskRequest) (*Watcher, error) {
	panic("not implemented")
}

func (mc *MockClient) RunURL(id string, envSlug string) string {
	panic("not implemented")
}
