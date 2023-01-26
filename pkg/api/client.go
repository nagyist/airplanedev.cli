// Package api implements Airplane HTTP API client.
package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/version"
	libapi "github.com/airplanedev/lib/pkg/api"
	libhttp "github.com/airplanedev/lib/pkg/api/http"
	"github.com/pkg/errors"
)

const (
	// Host is the default API host.
	Host = "api.airplane.dev"
)

// Client implements Airplane client.
type Client struct {
	ClientOpts
	http libhttp.Client
}

type ClientOpts struct {
	// Host is the API host to use.
	//
	// If empty, it uses the global `api.Host`.
	Host string

	// Token is the token to use for authentication.
	//
	// The token must be set, otherwise all methods will return an error.
	Token string

	// Extra information about what context the CLI is being used.
	// e.g. in a GitHub action.
	Source string

	// Alternative to token-based authn.
	APIKey string
	TeamID string
}

func NewClient(opts ClientOpts) Client {
	headers := map[string]string{
		"X-Airplane-Client-Kind":    "cli",
		"X-Airplane-Client-Version": version.Get(),
	}
	if opts.Source != "" {
		headers["X-Airplane-Client-Source"] = opts.Source
	}

	return Client{
		ClientOpts: opts,
		http: libhttp.NewClient(libhttp.ClientOpts{
			Headers:   headers,
			UserAgent: "airplane/cli/" + version.Get(),
			// Temporarily bump the default timeout to 30s.
			// TODO: revert to the default 10s after optimizing the long-tail of slow API endpoints.
			Timeout: 30 * time.Second,
			RequestLogHook: func(req *http.Request, attempt int) {
				msg := "requesting..."
				if attempt > 1 {
					msg = fmt.Sprintf("retrying... (attempt #%d)", attempt)
				}
				logger.Debug("%s %s: %s", req.Method, req.URL.Path, msg)
			},
			ResponseLogHook: func(resp *http.Response) {
				if !logger.EnableDebug {
					return
				}

				// Print out the response body for debugging. Reset resp.Body since we read it to completion.
				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				resp.Body = io.NopCloser(bytes.NewReader(body))

				if err != nil {
					logger.Debug("%s %s (%d): failed to read response body: %v", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, err)
				} else {
					b := string(body)
					if len(b) == 0 {
						b = "(no response)"
					}
					logger.Debug("%s %s (%d): %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, b)
				}
			},
		}),
	}
}

type APIClient interface {
	// GetTask fetches a task by slug. If the slug does not match a task, a *TaskMissingError is returned.
	GetTask(ctx context.Context, req libapi.GetTaskRequest) (res libapi.Task, err error)

	// GetTaskByID gets a task by ID.
	// TODO: Add an ID into libapi.GetTaskRequest so that we can just use GetTask instead of having this too.
	GetTaskByID(ctx context.Context, id string) (res libapi.Task, err error)

	// GetTaskMetadata fetches a task's metadata by slug. If the slug does not match a task, a *TaskMissingError is returned.
	GetTaskMetadata(ctx context.Context, slug string) (res libapi.TaskMetadata, err error)
	GetTaskReviewers(ctx context.Context, slug string) (res GetTaskReviewersResponse, err error)
	ListTasks(ctx context.Context, envSlug string) (res ListTasksResponse, err error)
	CreateTask(ctx context.Context, req CreateTaskRequest) (res CreateTaskResponse, err error)
	UpdateTask(ctx context.Context, req libapi.UpdateTaskRequest) (res UpdateTaskResponse, err error)
	RunTask(ctx context.Context, req RunTaskRequest) (RunTaskResponse, error)
	TaskURL(slug string, envSlug string) string

	GetRun(ctx context.Context, id string) (res GetRunResponse, err error)
	GetOutputs(ctx context.Context, runID string) (res GetOutputsResponse, err error)
	GetRunbook(ctx context.Context, runbookSlug string, envSlug string) (res GetRunbookResponse, err error)
	ListSessionBlocks(ctx context.Context, sessionID string) (res ListSessionBlocksResponse, err error)

	ListResources(ctx context.Context, envSlug string) (res libapi.ListResourcesResponse, err error)
	ListResourceMetadata(ctx context.Context) (res libapi.ListResourceMetadataResponse, err error)
	GetResource(ctx context.Context, req GetResourceRequest) (res libapi.GetResourceResponse, err error)

	SetConfig(ctx context.Context, req SetConfigRequest) (err error)
	GetConfig(ctx context.Context, req GetConfigRequest) (res GetConfigResponse, err error)
	ListConfigs(ctx context.Context, req ListConfigsRequest) (res ListConfigsResponse, err error)

	GetRegistryToken(ctx context.Context) (res RegistryTokenResponse, err error)

	GetDeploymentLogs(ctx context.Context, deploymentID string, prevToken string) (res GetDeploymentLogsResponse, err error)
	GetDeployment(ctx context.Context, id string) (res Deployment, err error)
	CreateDeployment(ctx context.Context, req CreateDeploymentRequest) (CreateDeploymentResponse, error)
	CancelDeployment(ctx context.Context, req CancelDeploymentRequest) error
	DeploymentURL(deploymentID string, envSlug string) string

	CreateBuildUpload(ctx context.Context, req libapi.CreateBuildUploadRequest) (res libapi.CreateBuildUploadResponse, err error)
	GenerateSignedURLs(ctx context.Context, envSlug string) (res GenerateSignedURLsResponse, err error)
	CreateUpload(ctx context.Context, req libapi.CreateUploadRequest) (res libapi.CreateUploadResponse, err error)
	GetUpload(ctx context.Context, uploadID string) (res libapi.GetUploadResponse, err error)

	GetView(ctx context.Context, req libapi.GetViewRequest) (libapi.View, error)
	CreateView(ctx context.Context, req libapi.CreateViewRequest) (libapi.View, error)
	CreateDemoDB(ctx context.Context, name string) (string, error)

	GetEnv(ctx context.Context, envSlug string) (libapi.Env, error)

	EvaluateTemplate(ctx context.Context, req libapi.EvaluateTemplateRequest) (res libapi.EvaluateTemplateResponse, err error)

	GetPermissions(ctx context.Context, taskSlug string, actions []string) (GetPermissionsResponse, error)

	// All methods below this point represent CLI-specific API operations, and not requests to api.airplane.dev.
	GetToken() string

	ListFlags(ctx context.Context) (ListFlagsResponse, error)

	GetWebHost(ctx context.Context) (string, error)

	GetUser(ctx context.Context, userID string) (GetUserResponse, error)

	GetTunnelToken(ctx context.Context) (GetTunnelTokenResponse, error)
}

var _ APIClient = Client{}
var _ libapi.IAPIClient = Client{}

// AppURL returns the app URL.
func (c Client) AppURL() *url.URL {
	apphost := c.host()
	apphost = strings.ReplaceAll(apphost, "api.airstage.app", "web.airstage.app")
	apphost = strings.ReplaceAll(apphost, "api", "app")
	u, _ := url.Parse(c.scheme() + apphost)
	return u
}

// HostURL returns the api URL, e.g. api.airstage.app
func (c Client) HostURL() string {
	return c.scheme() + c.host()
}

func (c Client) GetToken() string {
	return c.Token
}

func (c Client) TokenURL() string {
	u := c.AppURL()
	u.Path = "/cli/login"
	u.RawQuery = url.Values{"showToken": []string{"1"}}.Encode()
	return u.String()
}

func (c Client) LoginURL(uri string) string {
	u := c.AppURL()
	u.Path = "/cli/login"
	u.RawQuery = url.Values{"redirect": []string{uri}}.Encode()
	return u.String()
}

// LoginSuccessURL returns a URL showing a message that logging in was successful.
func (c Client) LoginSuccessURL() string {
	u := c.AppURL()
	u.Path = "/cli/success"
	return u.String()
}

// DeploymentURL returns a URL for a deployment.
func (c Client) DeploymentURL(deploymentID string, envSlug string) string {
	u := c.AppURL()
	u.Path = fmt.Sprintf("/deployments/%s", deploymentID)
	if envSlug != "" {
		u.RawQuery = url.Values{"__env": []string{envSlug}}.Encode()
	}
	return u.String()
}

// RunURL returns a run URL for a run ID.
func (c Client) RunURL(id string, envSlug string) string {
	u := c.AppURL()
	u.Path = "/runs/" + id
	if envSlug != "" {
		u.RawQuery = url.Values{"__env": []string{envSlug}}.Encode()
	}
	return u.String()
}

// TaskURL returns a task URL for a task slug.
func (c Client) TaskURL(slug string, envSlug string) string {
	u := c.AppURL()
	u.Path = "/t/" + slug
	if envSlug != "" {
		u.RawQuery = url.Values{"__env": []string{envSlug}}.Encode()
	}
	return u.String()
}

// AuthInfo responds with the currently authenticated details.
func (c Client) AuthInfo(ctx context.Context) (res AuthInfoResponse, err error) {
	err = c.get(ctx, "/auth/info", &res)
	return
}

// GetRegistryToken responds with the registry token.
func (c Client) GetRegistryToken(ctx context.Context) (res RegistryTokenResponse, err error) {
	err = c.post(ctx, "/registry/getToken", nil, &res)
	return
}

// CreateTask creates a task with the given request.
func (c Client) CreateTask(ctx context.Context, req CreateTaskRequest) (res CreateTaskResponse, err error) {
	err = c.post(ctx, encodeQueryString("/tasks/create", url.Values{
		"envSlug": []string{req.EnvSlug},
	}), req, &res)
	return
}

// UpdateTask updates a task with the given req.
func (c Client) UpdateTask(ctx context.Context, req libapi.UpdateTaskRequest) (res UpdateTaskResponse, err error) {
	err = c.post(ctx, "/tasks/update", req, &res)
	return
}

// ListTasks lists all tasks.
func (c Client) ListTasks(ctx context.Context, envSlug string) (res ListTasksResponse, err error) {
	err = c.get(ctx, encodeQueryString("/tasks/list", url.Values{
		"envSlug": []string{envSlug},
	}), &res)
	if err != nil {
		return
	}
	for j, t := range res.Tasks {
		res.Tasks[j].URL = c.TaskURL(t.Slug, envSlug)
	}
	return
}

func (c Client) ListFlags(ctx context.Context) (res ListFlagsResponse, err error) {
	err = c.get(ctx, "/flags/list", &res)
	if err != nil {
		return
	}
	return
}

// GetUniqueSlug gets a unique slug based on the given name.
func (c Client) GetUniqueSlug(ctx context.Context, name, preferredSlug string) (res GetUniqueSlugResponse, err error) {
	q := url.Values{
		"name": []string{name},
		"slug": []string{preferredSlug},
	}
	err = c.get(ctx, "/tasks/getUniqueSlug?"+q.Encode(), &res)
	return
}

// ListRuns lists most recent runs.
func (c Client) ListRuns(ctx context.Context, req ListRunsRequest) (ListRunsResponse, error) {
	pageLimit := 100
	if req.Limit > 0 && req.Limit < 100 {
		// If a user provides a smaller limit, fetch exactly that many items.
		pageLimit = req.Limit
	}

	q := url.Values{
		"page":    []string{strconv.FormatInt(int64(req.Page), 10)},
		"taskID":  []string{req.TaskID},
		"limit":   []string{strconv.FormatInt(int64(pageLimit), 10)},
		"envSlug": []string{req.EnvSlug},
	}
	if !req.Since.IsZero() {
		q.Set("since", req.Since.Format(time.RFC3339))
	}
	if !req.Until.IsZero() {
		q.Set("until", req.Until.Format(time.RFC3339))
	}

	var resp ListRunsResponse
	var page ListRunsResponse
	var i int
	for {
		q.Set("page", strconv.FormatInt(int64(i), 10))
		i++
		if err := c.get(ctx, encodeQueryString("/runs/list", q), &page); err != nil {
			return ListRunsResponse{}, err
		}
		runs := page.Runs
		if req.Limit > 0 && len(resp.Runs)+len(runs) > req.Limit {
			// Truncate the response if we over-fetched items:
			runs = runs[:req.Limit-len(resp.Runs)]
		}
		resp.Runs = append(resp.Runs, runs...)

		// There are no more items to fetch:
		if len(page.Runs) != pageLimit {
			break
		}
		// We have reached the requested limit of items to fetch:
		if req.Limit > 0 && len(resp.Runs) == req.Limit {
			break
		}
	}

	return resp, nil
}

// RunTask runs a task.
func (c Client) RunTask(ctx context.Context, req RunTaskRequest) (RunTaskResponse, error) {
	var res RunTaskResponse
	if err := c.post(ctx, encodeQueryString("/tasks/execute", url.Values{
		"envSlug": []string{req.EnvSlug},
	}), req, &res); err != nil {
		var errsc libhttp.ErrStatusCode
		if errors.As(err, &errsc) && errsc.StatusCode == 404 {
			if req.TaskSlug != nil {
				return res, &libapi.TaskMissingError{
					AppURL: c.AppURL().String(),
					Slug:   *req.TaskSlug,
				}
			}
		} else {
			return RunTaskResponse{}, err
		}
	}

	return res, nil
}

// Watcher runs a task with the given arguments and returns a run watcher.
func (c Client) Watcher(ctx context.Context, req RunTaskRequest) (*Watcher, error) {
	resp, err := c.RunTask(ctx, req)
	if err != nil {
		return nil, err
	}
	return newWatcher(ctx, c, resp.RunID), nil
}

// GetRun returns a run by id.
func (c Client) GetRun(ctx context.Context, id string) (res GetRunResponse, err error) {
	q := url.Values{"runID": []string{id}}
	err = c.get(ctx, "/runs/get?"+q.Encode(), &res)
	return
}

// GetLogs returns the logs by runID and since timestamp.
func (c Client) GetLogs(ctx context.Context, runID, prevToken string) (res GetLogsResponse, err error) {
	q := url.Values{"runID": []string{runID}}
	if prevToken != "" {
		q.Set("prev_token", prevToken)
	}
	if logger.EnableDebug {
		q.Set("level", "debug")
	}
	err = c.get(ctx, "/runs/getLogs?"+q.Encode(), &res)
	return
}

// GetOutputs returns the outputs by runID.
func (c Client) GetOutputs(ctx context.Context, runID string) (res GetOutputsResponse, err error) {
	q := url.Values{"runID": []string{runID}}
	err = c.get(ctx, "/runs/getOutputs?"+q.Encode(), &res)
	return
}

// GetRunbook returns the details of a runbook by slug.
func (c Client) GetRunbook(ctx context.Context, runbookSlug string, envSlug string) (res GetRunbookResponse, err error) {
	q := url.Values{"runbookSlug": []string{runbookSlug}, "envSlug": []string{envSlug}}
	err = c.get(ctx, "/runbooks/get?"+q.Encode(), &res)
	return
}

// GetOutputs returns the outputs by runID.
func (c Client) ListSessionBlocks(ctx context.Context, sessionID string) (
	res ListSessionBlocksResponse,
	err error,
) {
	// TODO: list session blocks will error when unmarshaling if there is a template constraint
	// in the parameter option, until the Parameter struct is updated in lib to match airport
	q := url.Values{"sessionID": []string{sessionID}}
	err = c.get(ctx, "/sessions/listBlocks?"+q.Encode(), &res)
	return
}

// GetTask fetches a task by slug. If the slug does not match a task, a *TaskMissingError is returned.
func (c Client) GetTask(ctx context.Context, req libapi.GetTaskRequest) (res libapi.Task, err error) {
	err = c.get(ctx, encodeQueryString("/tasks/get", url.Values{
		"slug":    []string{req.Slug},
		"envSlug": []string{req.EnvSlug},
	}), &res)

	var errsc libhttp.ErrStatusCode
	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
		return res, &libapi.TaskMissingError{
			AppURL: c.AppURL().String(),
			Slug:   req.Slug,
		}
	}

	if err != nil {
		return
	}
	res.URL = c.TaskURL(res.Slug, req.EnvSlug)
	return
}

func (c Client) GetTaskByID(ctx context.Context, id string) (res libapi.Task, err error) {
	err = c.get(ctx, encodeQueryString("/tasks/get", url.Values{
		"id": []string{id},
	}), &res)

	var errsc libhttp.ErrStatusCode
	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
		return res, &libapi.TaskMissingError{
			AppURL: c.AppURL().String(),
		}
	}
	if err != nil {
		return
	}
	return
}

// GetTaskMetadata fetches a task's metadata by slug. If the slug does not match a task, a *TaskMissingError is returned.
func (c Client) GetTaskMetadata(ctx context.Context, slug string) (res libapi.TaskMetadata, err error) {
	err = c.get(ctx, encodeQueryString("/tasks/getMetadata", url.Values{
		"slug": []string{slug},
	}), &res)

	var errsc libhttp.ErrStatusCode
	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
		return res, &libapi.TaskMissingError{
			AppURL: c.AppURL().String(),
			Slug:   slug,
		}
	}

	return
}

// GetTaskReviewers fetches a task and reviewers by slug. If the slug does not match a task, a *TaskMissingError is returned.
func (c Client) GetTaskReviewers(ctx context.Context, slug string) (res GetTaskReviewersResponse, err error) {
	err = c.get(ctx, encodeQueryString("/tasks/getTaskReviewers", url.Values{
		"taskSlug": []string{slug},
	}), &res)

	var errsc libhttp.ErrStatusCode
	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
		return res, &libapi.TaskMissingError{
			AppURL: c.AppURL().String(),
			Slug:   slug,
		}
	}

	return
}

// GetView fetches a view. If the view does not exist, a *ViewMissingError is returned.
func (c Client) GetView(ctx context.Context, req libapi.GetViewRequest) (res libapi.View, err error) {
	err = c.get(ctx, encodeQueryString("/views/get", url.Values{
		"slug": []string{req.Slug},
		"id":   []string{req.ID},
	}), &res)

	var errsc libhttp.ErrStatusCode
	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
		return res, &libapi.ViewMissingError{
			AppURL: c.AppURL().String(),
			Slug:   req.Slug,
		}
	}

	return
}

func (c Client) CreateView(ctx context.Context, req libapi.CreateViewRequest) (res libapi.View, err error) {
	err = c.post(ctx, "/views/create", req, &res)
	return
}

func (c Client) CreateDemoDB(ctx context.Context, name string) (string, error) {
	reply := struct {
		ResourceID string `json:"resourceID"`
	}{}
	err := c.post(ctx, "/resources/createDemoDB", CreateDemoDBRequest{
		Name: name,
	}, &reply)
	if err != nil {
		return "", err
	}
	return reply.ResourceID, nil
}

func (c Client) ResetDemoDB(ctx context.Context) (string, error) {
	reply := struct {
		ResourceID string `json:"resourceID"`
	}{}
	err := c.post(ctx, "/resources/resetDemoDB", nil, &reply)
	if err != nil {
		return "", err
	}
	return reply.ResourceID, nil
}

// GetConfig returns a config by name and tag.
func (c Client) GetConfig(ctx context.Context, req GetConfigRequest) (res GetConfigResponse, err error) {
	err = c.post(ctx, encodeQueryString("/configs/get", url.Values{
		"envSlug": []string{req.EnvSlug},
	}), req, &res)
	return
}

// SetConfig sets a config, creating it if new and updating it if already exists.
func (c Client) SetConfig(ctx context.Context, req SetConfigRequest) (err error) {
	err = c.post(ctx, encodeQueryString("/configs/set", url.Values{
		"envSlug": []string{req.EnvSlug},
	}), req, nil)
	return
}

// ListConfigs returns a config by name and tag.
func (c Client) ListConfigs(ctx context.Context, req ListConfigsRequest) (res ListConfigsResponse, err error) {
	err = c.get(ctx, encodeQueryString("/configs/list", url.Values{
		"names":       req.Names,
		"showSecrets": []string{strconv.FormatBool(req.ShowSecrets)},
		"envSlug":     []string{req.EnvSlug},
	}), &res)
	return
}

// GetDeployment returns a deployment.
func (c Client) GetDeployment(ctx context.Context, id string) (res Deployment, err error) {
	q := url.Values{"id": []string{id}}
	err = c.get(ctx, "/deployments/get?"+q.Encode(), &res)
	return
}

// CreateBuildUpload creates an Airplane upload and returns metadata about it.
func (c Client) CreateBuildUpload(ctx context.Context, req libapi.CreateBuildUploadRequest) (res libapi.CreateBuildUploadResponse, err error) {
	err = c.post(ctx, "/builds/createUpload", req, &res)
	return
}

// CreateAPIKey creates a new API key and returns data about it.
func (c Client) CreateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (res CreateAPIKeyResponse, err error) {
	err = c.post(ctx, "/apiKeys/create", req, &res)
	return
}

// ListAPIKeys lists API keys.
func (c Client) ListAPIKeys(ctx context.Context) (res ListAPIKeysResponse, err error) {
	err = c.get(ctx, "/apiKeys/list", &res)
	return
}

// DeleteAPIKey deletes an API key.
func (c Client) DeleteAPIKey(ctx context.Context, req DeleteAPIKeyRequest) (err error) {
	err = c.post(ctx, "/apiKeys/delete", req, nil)
	return
}

func (c Client) CreateDeployment(ctx context.Context, req CreateDeploymentRequest) (res CreateDeploymentResponse, err error) {
	err = c.post(ctx, encodeQueryString("/deployments/create", url.Values{
		"envSlug": []string{req.EnvSlug},
	}), req, &res)
	return
}

func (c Client) CancelDeployment(ctx context.Context, req CancelDeploymentRequest) error {
	return c.post(ctx, "/deployments/cancel", req, nil)
}

func (c Client) GetDeploymentLogs(ctx context.Context, deploymentID string, prevToken string) (res GetDeploymentLogsResponse, err error) {
	q := url.Values{
		"id": []string{deploymentID},
	}
	if logger.EnableDebug {
		q.Set("level", "debug")
	}
	if prevToken != "" {
		q.Set("prevToken", prevToken)
	}
	err = c.get(ctx, encodeQueryString("/deployments/getLogs", q), &res)
	return
}

func (c Client) ListResources(ctx context.Context, envSlug string) (res libapi.ListResourcesResponse, err error) {
	err = c.get(ctx, encodeQueryString("/resources/list", url.Values{
		"envSlug": []string{envSlug},
	}), &res)
	return
}

func (c Client) ListResourceMetadata(ctx context.Context) (res libapi.ListResourceMetadataResponse, err error) {
	err = c.get(ctx, "/resources/listMetadata", &res)
	return
}

func (c Client) GetResource(ctx context.Context, req GetResourceRequest) (res libapi.GetResourceResponse, err error) {
	err = c.get(ctx, encodeQueryString("/resources/get", url.Values{
		"id":                   []string{req.ID},
		"slug":                 []string{req.Slug},
		"envSlug":              []string{req.EnvSlug},
		"includeSensitiveData": []string{strconv.FormatBool(req.IncludeSensitiveData)},
	}), &res)
	var errsc libhttp.ErrStatusCode
	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
		return res, libapi.ResourceMissingError{
			AppURL: c.AppURL().String(),
			Slug:   req.Slug,
		}
	}
	return
}

func (c Client) GetEnv(ctx context.Context, envSlug string) (res libapi.Env, err error) {
	err = c.get(ctx, encodeQueryString("/envs/get", url.Values{
		"slug": []string{envSlug},
	}), &res)
	return
}

func (c Client) EvaluateTemplate(ctx context.Context, req libapi.EvaluateTemplateRequest) (res libapi.EvaluateTemplateResponse, err error) {
	err = c.post(ctx, "/templates/evaluate", req, &res)
	return
}

func (c Client) GetPermissions(ctx context.Context, taskSlug string, actions []string) (res GetPermissionsResponse, err error) {
	err = c.get(ctx, encodeQueryString("/permissions/get", url.Values{
		"task_slug": []string{taskSlug},
		"actions":   actions,
	}), &res)
	return
}

func (c Client) CreateUpload(ctx context.Context, req libapi.CreateUploadRequest) (res libapi.CreateUploadResponse, err error) {
	err = c.post(ctx, "/uploads/create", req, &res)
	return
}

func (c Client) GetUpload(ctx context.Context, uploadID string) (res libapi.GetUploadResponse, err error) {
	err = c.get(ctx, encodeQueryString("/uploads/get", url.Values{
		"id": []string{uploadID},
	}), &res)
	return
}

func (c Client) GenerateSignedURLs(ctx context.Context, envSlug string) (res GenerateSignedURLsResponse, err error) {
	err = c.get(ctx, encodeQueryString("/uploads/generateSignedURLs", url.Values{
		"envSlug": []string{envSlug},
	}), &res)
	return
}

func (c Client) GetWebHost(ctx context.Context) (webHost string, err error) {
	err = c.get(ctx, "/hosts/web", &webHost)
	return
}

func (c Client) GetUser(ctx context.Context, userID string) (res GetUserResponse, err error) {
	err = c.get(ctx, encodeQueryString("/users/get", url.Values{
		"userID": []string{userID},
	}), &res)
	return
}

func (c Client) GetTunnelToken(ctx context.Context) (res GetTunnelTokenResponse, err error) {
	err = c.get(ctx, "/studio/tunnelToken/get", &res)
	return
}

func (c Client) headers() (map[string]string, error) {
	headers := map[string]string{}
	if c.Token != "" {
		headers["X-Airplane-Token"] = c.Token
	} else if c.APIKey != "" {
		headers["X-Airplane-API-Key"] = c.APIKey
		if c.TeamID == "" {
			return nil, errors.New("team ID is missing")
		}
		headers["X-Team-ID"] = c.TeamID
	} else {
		return nil, errors.Errorf("authentication is missing: %s", c.APIKey)
	}

	return headers, nil
}

func (c Client) get(ctx context.Context, path string, reply interface{}) error {
	headers, err := c.headers()
	if err != nil {
		return err
	}

	pathname := "/v0" + path
	url := c.scheme() + c.host() + pathname
	err = c.http.GetJSON(ctx, url, reply, libhttp.ReqOpts{
		Headers: headers,
	})
	if err != nil {
		logger.Debug("GET %s: request failed: %v", pathname, err)
		return err
	}

	return nil
}

func (c Client) post(ctx context.Context, path string, payload, reply interface{}) error {
	headers, err := c.headers()
	if err != nil {
		return err
	}

	pathname := "/v0" + path
	url := c.scheme() + c.host() + pathname
	err = c.http.PostJSON(ctx, url, payload, reply, libhttp.ReqOpts{
		Headers: headers,
	})
	if err != nil {
		logger.Debug("POST %s: request failed: %v", pathname, err)
		return err
	}

	return nil
}

// Host returns the configured endpoint.
func (c Client) host() string {
	if c.Host != "" {
		return c.Host
	}
	return Host
}

var httpHosts = []string{
	"localhost",
	"127.0.0.1",
	"host.docker.internal",
	"172.17.0.1", // Docker for linux
	"api",
}

func (c Client) scheme() string {
	if c.Host == Host {
		return "https://"
	}

	// If the host didn't come with a scheme, force a "//" in front of it.
	host := c.Host
	if !strings.HasPrefix(host, "http") {
		host = fmt.Sprintf("//%s", host)
	}
	u, err := url.Parse(host)
	if err != nil {
		return "https://"
	}

	for _, httpHost := range httpHosts {
		if u.Hostname() == httpHost {
			return "http://"
		}
	}

	return "https://"
}

// encodeURL is a helper for encoding a set of query parameters onto a URL.
//
// If a query parameter is an empty string, it will be excluded from the
// encoded query string.
func encodeQueryString(path string, params url.Values) string {
	updatedParams := url.Values{}
	for k, v := range params {
		// Remove any query parameters
		if len(v) > 1 || (len(v) == 1 && len(v[0]) > 0) {
			updatedParams[k] = v
		}
	}

	if len(updatedParams) == 0 {
		return path
	}

	return path + "?" + updatedParams.Encode()
}
