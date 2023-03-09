package apiint

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/configs"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/resources"
	"github.com/airplanedev/cli/pkg/server/handlers"
	"github.com/airplanedev/cli/pkg/server/outputs"
	"github.com/airplanedev/cli/pkg/server/state"
	serverutils "github.com/airplanedev/cli/pkg/server/utils"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	libapi "github.com/airplanedev/lib/pkg/api"
	libhttp "github.com/airplanedev/lib/pkg/api/http"
	"github.com/airplanedev/lib/pkg/build"
	libresources "github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/conversion"
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/gorilla/mux"
	"github.com/gosimple/slug"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
)

// AttachInternalAPIRoutes attaches a minimal subset of the internal Airplane API endpoints that are necessary for the
// previewer
func AttachInternalAPIRoutes(r *mux.Router, state *state.State) {
	const basePath = "/i/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/resources/create", handlers.WithBody(state, CreateResourceHandler)).Methods("POST", "OPTIONS")
	r.Handle("/resources/get", handlers.New(state, GetResourceHandler)).Methods("GET", "OPTIONS")
	r.Handle("/resources/list", handlers.New(state, ListResourcesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/resources/update", handlers.WithBody(state, UpdateResourceHandler)).Methods("POST", "OPTIONS")
	r.Handle("/resources/delete", handlers.WithBody(state, DeleteResourceHandler)).Methods("POST", "OPTIONS")
	r.Handle("/resources/isSlugAvailable", handlers.New(state, IsResourceSlugAvailableHandler)).Methods("GET", "OPTIONS")

	r.Handle("/displays/list", handlers.New(state, ListDisplaysHandler)).Methods("GET", "OPTIONS")

	r.Handle("/prompts/list", handlers.New(state, ListPromptHandler)).Methods("GET", "OPTIONS")
	r.Handle("/prompts/submit", handlers.WithBody(state, SubmitPromptHandler)).Methods("POST", "OPTIONS")

	r.Handle("/sleeps/list", handlers.New(state, ListSleepsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/sleeps/skip", handlers.WithBody(state, SkipSleepHandler)).Methods("POST", "OPTIONS")

	r.Handle("/runs/get", handlers.New(state, GetRunHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/getDescendants", handlers.New(state, GetDescendantsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/list", handlers.New(state, ListRunsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/cancel", handlers.WithBody(state, CancelRunHandler)).Methods("POST", "OPTIONS")
	r.Handle("/runs/getOutputs", handlers.New(state, outputs.GetOutputsHandler)).Methods("GET", "OPTIONS")

	r.Handle("/tasks/get", handlers.New(state, GetTaskInfoHandler)).Methods("GET", "OPTIONS")
	r.Handle("/views/get", handlers.New(state, GetViewInfoHandler)).Methods("GET", "OPTIONS")

	r.Handle("/users/get", handlers.New(state, GetUserHandler)).Methods("GET", "OPTIONS")

	r.Handle("/configs/get", handlers.New(state, GetConfigHandler)).Methods("GET", "OPTIONS")
	r.Handle("/configs/upsert", handlers.WithBody(state, UpsertConfigHandler)).Methods("POST", "OPTIONS")
	r.Handle("/configs/delete", handlers.WithBody(state, DeleteConfigHandler)).Methods("POST", "OPTIONS")
	r.Handle("/configs/list", handlers.New(state, ListConfigsHandler)).Methods("GET", "OPTIONS")

	r.Handle("/uploads/create", handlers.WithBody(state, CreateUploadHandler)).Methods("POST", "OPTIONS")
	r.Handle("/uploads/get", handlers.WithBody(state, GetUploadHandler)).Methods("POST", "OPTIONS") // Our web app expects POST for this endpoint

	r.Handle("/envs/list", handlers.New(state, ListEnvsHandler)).Methods("GET", "OPTIONS")
}

type CreateResourceRequest struct {
	Name           string                    `json:"name"`
	Slug           string                    `json:"slug"`
	Kind           libresources.ResourceKind `json:"kind"`
	ExportResource libresources.Resource     `json:"resource"`
}

func (r *CreateResourceRequest) UnmarshalJSON(buf []byte) error {
	var raw struct {
		Name           string                    `json:"name"`
		Slug           string                    `json:"slug"`
		Kind           libresources.ResourceKind `json:"kind"`
		ExportResource map[string]interface{}    `json:"resource"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	var export libresources.Resource
	var err error
	if raw.ExportResource != nil {
		export, err = libresources.GetResource(libresources.ResourceKind(raw.Kind), raw.ExportResource)
		if err != nil {
			return err
		}
	}

	r.Name = raw.Name
	r.Slug = raw.Slug
	r.Kind = raw.Kind
	r.ExportResource = export

	return nil
}

type CreateResourceResponse struct {
	ResourceID string `json:"resourceID"`
}

// CreateResourceHandler handles requests to the /i/resources/get endpoint
func CreateResourceHandler(ctx context.Context, state *state.State, r *http.Request, req CreateResourceRequest) (CreateResourceResponse, error) {
	resourceSlug := req.Slug
	var err error
	if resourceSlug == "" {
		if resourceSlug, err = utils.GetUniqueSlug(utils.GetUniqueSlugRequest{
			Slug: slug.Make(req.Name),
			SlugExists: func(slug string) (bool, error) {
				_, ok := state.DevConfig.Resources[slug]
				return ok, nil
			},
		}); err != nil {
			return CreateResourceResponse{}, errors.Errorf("could not generate unique resource slug: %s", err.Error())
		}
	} else {
		if _, ok := state.DevConfig.Resources[resourceSlug]; ok {
			return CreateResourceResponse{}, libhttp.NewErrBadRequest("Resource with slug %s already exists", resourceSlug)
		}
	}

	if err := req.ExportResource.Calculate(); err != nil {
		return CreateResourceResponse{}, errors.Wrap(err, "computing calculated fields")
	}

	resource := req.ExportResource
	id := utils.GenerateDevResourceID(resourceSlug)
	if err := resource.UpdateBaseResource(libresources.BaseResource{
		ID:   id,
		Slug: resourceSlug,
		Kind: req.Kind,
		Name: req.Name,
	}); err != nil {
		return CreateResourceResponse{}, errors.Wrap(err, "updating base resource")
	}

	if err := state.DevConfig.SetResource(resourceSlug, resource); err != nil {
		return CreateResourceResponse{}, errors.Wrap(err, "setting resource")
	}

	return CreateResourceResponse{ResourceID: id}, nil
}

type GetResourceResponse struct {
	Resource kind_configs.InternalResource
}

// GetResourceHandler handles requests to the /i/resources/get endpoint
func GetResourceHandler(ctx context.Context, state *state.State, r *http.Request) (GetResourceResponse, error) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		return GetResourceResponse{}, libhttp.NewErrBadRequest("resource slug was not supplied")
	}

	for s, r := range state.DevConfig.Resources {
		if s == slug {
			internalResource, err := conversion.ConvertToInternalResource(r.Resource)
			if err != nil {
				return GetResourceResponse{}, errors.Wrap(err, "converting to internal resource")
			}
			return GetResourceResponse{Resource: internalResource}, nil
		}
	}

	return GetResourceResponse{}, libhttp.NewErrNotFound("resource with slug %s is not in dev config file", slug)
}

type APIResourceWithEnv struct {
	libapi.Resource
	Remote bool       `json:"remote"`
	Env    libapi.Env `json:"env"`
}

type ListResourcesResponse struct {
	Resources []APIResourceWithEnv `json:"resources"`
}

// ListResourcesHandler handles requests to the /i/resources/list endpoint
func ListResourcesHandler(ctx context.Context, state *state.State, r *http.Request) (ListResourcesResponse, error) {
	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	resourcesWithEnv := make([]APIResourceWithEnv, 0)
	for slug, r := range state.DevConfig.Resources {
		resourcesWithEnv = append(resourcesWithEnv, APIResourceWithEnv{
			Resource: libapi.Resource{
				ID:                r.Resource.GetID(),
				Name:              r.Resource.GetName(),
				Slug:              slug,
				Kind:              libapi.ResourceKind(r.Resource.Kind()),
				ExportResource:    r.Resource,
				CanUseResource:    true,
				CanUpdateResource: true,
			},
			Remote: false,
			Env:    env.NewLocalEnv(),
		})
	}

	remoteResources, err := resources.ListRemoteResources(ctx, state.RemoteClient, envSlug)
	if err == nil {
		remoteEnv, err := state.GetEnv(ctx, pointers.ToString(envSlug))
		if err != nil {
			return ListResourcesResponse{}, err
		}
		for _, r := range remoteResources {
			// This is purely so we can display remote resource information in the local dev studio. The remote list
			// resources endpoint doesn't return CanUseResource or CanUpdateResource, and so we set them to true here.
			r.CanUseResource = true
			r.CanUpdateResource = true
			resourcesWithEnv = append(resourcesWithEnv, APIResourceWithEnv{
				Resource: r,
				Remote:   true,
				Env:      remoteEnv,
			})
		}
	} else {
		logger.Error("error fetching remote resources: %v", err)
	}

	return ListResourcesResponse{
		Resources: resourcesWithEnv,
	}, nil
}

type UpdateResourceRequest struct {
	ID             string                    `json:"id"`
	Slug           string                    `json:"slug"`
	Name           string                    `json:"name"`
	Kind           libresources.ResourceKind `json:"kind"`
	ExportResource libresources.Resource     `json:"resource"`
}

func (r *UpdateResourceRequest) UnmarshalJSON(buf []byte) error {
	var raw struct {
		ID             string                    `json:"id"`
		Slug           string                    `json:"slug"`
		Name           string                    `json:"name"`
		Kind           libresources.ResourceKind `json:"kind"`
		ExportResource map[string]interface{}    `json:"resource"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	var export libresources.Resource
	var err error
	if raw.ExportResource != nil {
		export, err = libresources.GetResource(raw.Kind, raw.ExportResource)
		if err != nil {
			return err
		}
	}

	r.ID = raw.ID
	r.Slug = raw.Slug
	r.Name = raw.Name
	r.Kind = raw.Kind
	r.ExportResource = export

	return nil
}

type UpdateResourceResponse struct {
	ResourceID string `json:"resourceID"`
}

// UpdateResourceHandler handles requests to the /i/resources/get endpoint
func UpdateResourceHandler(ctx context.Context, state *state.State, r *http.Request, req UpdateResourceRequest) (UpdateResourceResponse, error) {
	// Check if resource exists in dev config file.
	var foundResource bool
	var oldSlug string
	var resource libresources.Resource
	for configSlug, configResource := range state.DevConfig.Resources {
		// We can't rely on the slug for existence since it may have changed.
		if configResource.Resource.GetID() == req.ID {
			foundResource = true
			resource = configResource.Resource
			oldSlug = configSlug
			break
		}
	}
	if !foundResource {
		return UpdateResourceResponse{}, libhttp.NewErrNotFound("resource with slug %s not found in dev config file", req.Slug)
	}

	if err := resource.Update(req.ExportResource); err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "updating resource")
	}
	if err := resource.UpdateBaseResource(libresources.BaseResource{
		Slug: req.Slug,
		Name: req.Name,
	}); err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "updating base resoruce")
	}

	// Remove the old resource first - we need to do this since DevConfig.Resources is a mapping from slug to resource,
	// and if the update resource request involves updating the slug, we don't want to leave the old resource (under the
	// old slug) in the dev config file.
	if err := state.DevConfig.DeleteResource(oldSlug); err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "deleting old resource")
	}

	if err := state.DevConfig.SetResource(req.Slug, resource); err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "setting resource")
	}

	return UpdateResourceResponse{
		ResourceID: resource.GetID(),
	}, nil
}

type DeleteResourceRequest struct {
	ID string `json:"id"`
}

// DeleteResourceHandler handles requests to the /i/resources/delete endpoint
// The web app does utilize the response of the resource deletion handler.
func DeleteResourceHandler(ctx context.Context, state *state.State, r *http.Request, req DeleteResourceRequest) (struct{}, error) {
	id := req.ID
	for _, r := range state.DevConfig.Resources {
		if r.Resource.GetID() == id {
			if err := state.DevConfig.DeleteResource(r.Resource.GetSlug()); err != nil {
				return struct{}{}, errors.Wrap(err, "removing resource from dev config")
			}
			return struct{}{}, nil
		}
	}

	return struct{}{}, libhttp.NewErrNotFound("resource with id %q does not exist in dev config file", id)
}

type IsResourceSlugAvailableResponse struct {
	Available bool `json:"available"`
}

// IsResourceSlugAvailableHandler handles requests to the /i/resources/isSlugAvailable endpoint
func IsResourceSlugAvailableHandler(ctx context.Context, state *state.State, r *http.Request) (IsResourceSlugAvailableResponse, error) {
	slug := r.URL.Query().Get("slug")
	configResource, ok := state.DevConfig.Resources[slug]
	return IsResourceSlugAvailableResponse{
		Available: !ok || configResource.Resource.GetID() == r.URL.Query().Get("id"),
	}, nil
}

type ListDisplaysResponse struct {
	Displays []libapi.Display `json:"displays"`
}

func ListDisplaysHandler(ctx context.Context, state *state.State, r *http.Request) (ListDisplaysResponse, error) {
	runID := r.URL.Query().Get("runID")
	run, err := state.GetRunInternal(ctx, runID)
	if err != nil {
		return ListDisplaysResponse{}, err
	}

	return ListDisplaysResponse{
		Displays: append([]libapi.Display{}, run.Displays...),
	}, nil
}

type ListPromptsResponse struct {
	Prompts []libapi.Prompt `json:"prompts"`
}

func ListPromptHandler(ctx context.Context, state *state.State, r *http.Request) (ListPromptsResponse, error) {
	runID := r.URL.Query().Get("runID")
	if runID == "" {
		return ListPromptsResponse{}, libhttp.NewErrBadRequest("runID is required")
	}

	run, err := state.GetRunInternal(ctx, runID)
	if err != nil {
		return ListPromptsResponse{}, err
	}

	return ListPromptsResponse{Prompts: run.Prompts}, nil
}

type SubmitPromptRequest struct {
	ID          string                 `json:"id"`
	Values      map[string]interface{} `json:"values"`
	RunID       string                 `json:"runID"`
	SubmittedBy *string                `json:"-"`
}

type PromptResponse struct {
	ID string `json:"id"`
}

func SubmitPromptHandler(ctx context.Context, state *state.State, r *http.Request, req SubmitPromptRequest) (PromptResponse, error) {
	if req.ID == "" {
		return PromptResponse{}, libhttp.NewErrBadRequest("prompt ID is required")
	}
	if req.RunID == "" {
		return PromptResponse{}, libhttp.NewErrBadRequest("run ID is required")
	}

	userID := cli.ParseTokenForAnalytics(state.RemoteClient.GetToken()).UserID

	_, err := state.UpdateRun(req.RunID, func(run *dev.LocalRun) error {
		for i := range run.Prompts {
			if run.Prompts[i].ID == req.ID {
				now := time.Now()
				run.Prompts[i].SubmittedAt = &now
				run.Prompts[i].Values = req.Values
				run.Prompts[i].SubmittedBy = &userID

				// Check if the run is still waiting for user input.
				run.IsWaitingForUser = false
				for _, prompt := range run.Prompts {
					run.IsWaitingForUser = run.IsWaitingForUser || prompt.SubmittedAt == nil
				}

				return nil
			}
		}
		return libhttp.NewErrNotFound("prompt does not exist")
	})
	if err != nil {
		return PromptResponse{}, err
	}
	return PromptResponse{ID: req.ID}, nil
}

type GetDescendantsResponse struct {
	Descendants     []dev.LocalRun `json:"descendants"`
	DescendantTasks []libapi.Task  `json:"descendantTasks"`
}

func GetDescendantsHandler(ctx context.Context, state *state.State, r *http.Request) (GetDescendantsResponse, error) {
	runID := r.URL.Query().Get("runID")
	if runID == "" {
		return GetDescendantsResponse{}, libhttp.NewErrBadRequest("runID cannot be empty")
	}

	descendants, err := state.GetRunDescendants(ctx, runID)
	if err != nil {
		return GetDescendantsResponse{}, err
	}

	processedDescendants := make([]dev.LocalRun, len(descendants))
	descendantTasks := []libapi.Task{}
	taskIDsSeen := map[string]struct{}{}

	for i, descendant := range descendants {
		if descendant.Remote {
			resp, err := state.RemoteClient.GetRun(ctx, descendant.RunID)
			if err != nil {
				return GetDescendantsResponse{}, errors.Wrap(err, "getting remote run")
			}

			run := resp.Run

			descendant = dev.FromRemoteRun(run)
			descendant.ParentID = runID
			if _, ok := taskIDsSeen[run.TaskID]; !ok {
				task, err := state.RemoteClient.GetTaskByID(ctx, run.TaskID)
				if err != nil {
					return GetDescendantsResponse{}, errors.Wrap(err, "getting remote task")
				}
				descendantTasks = append(descendantTasks, task)
				taskIDsSeen[run.TaskID] = struct{}{}
			}
		} else {
			// There is no task ID for local task revisions so we use the slug
			taskID := descendant.TaskRevision.Def.GetSlug()
			if _, ok := taskIDsSeen[taskID]; !ok {
				utr, err := descendant.TaskRevision.Def.GetUpdateTaskRequest(ctx, state.LocalClient, false)
				if err != nil {
					return GetDescendantsResponse{}, errors.Errorf("error getting task %s", descendant.TaskRevision.Def.GetSlug())
				}
				localTask := libapi.Task{
					ID:          taskID,
					Name:        descendant.TaskRevision.Def.GetName(),
					Slug:        descendant.TaskRevision.Def.GetSlug(),
					Description: descendant.TaskRevision.Def.GetDescription(),
					Parameters:  utr.Parameters,
				}
				descendantTasks = append(descendantTasks, localTask)
				taskIDsSeen[taskID] = struct{}{}
			}
		}
		processedDescendants[i] = descendant
	}

	return GetDescendantsResponse{
		Descendants:     processedDescendants,
		DescendantTasks: descendantTasks,
	}, nil
}

func GetUserHandler(ctx context.Context, state *state.State, r *http.Request) (api.GetUserResponse, error) {
	userID := r.URL.Query().Get("userID")
	if userID == "" {
		return api.GetUserResponse{}, libhttp.NewErrBadRequest("userID cannot be empty")
	}

	resp, err := state.RemoteClient.GetUser(ctx, userID)
	if err != nil {
		logger.Debug("error getting user: %v", err)
		return api.GetUserResponse{
			User: DefaultUser(userID),
		}, nil
	}

	user := resp.User
	return api.GetUserResponse{
		User: api.User{
			ID:        userID,
			Email:     user.Email,
			Name:      user.Name,
			AvatarURL: user.AvatarURL,
		},
	}, nil
}

type GetRunResponse struct {
	Run  dev.LocalRun `json:"run"`
	Task *libapi.Task `json:"task"`
}

func GetRunHandler(ctx context.Context, state *state.State, r *http.Request) (GetRunResponse, error) {
	runID := r.URL.Query().Get("id")
	if runID == "" {
		runID = r.URL.Query().Get("runID")
	}
	run, err := state.GetRun(ctx, runID)
	if err != nil {
		return GetRunResponse{}, err
	}
	if run.Remote {
		resp, err := state.RemoteClient.GetRun(ctx, runID)
		if err != nil {
			return GetRunResponse{}, errors.Wrap(err, "getting remote run")
		}
		run = dev.FromRemoteRun(resp.Run)
		response := GetRunResponse{Run: run}
		task, err := state.RemoteClient.GetTaskByID(ctx, run.TaskID)
		if err != nil {
			return GetRunResponse{}, errors.Wrap(err, "getting remote task")
		}
		response.Task = &task
		return response, nil
	}

	utr, err := run.TaskRevision.Def.GetUpdateTaskRequest(ctx, state.LocalClient, false)
	run.Parameters = &utr.Parameters
	if err != nil {
		logger.Error("Encountered error while getting task info: %v", err)
		return GetRunResponse{}, errors.Errorf("error getting task %s", run.TaskRevision.Def.GetSlug())
	}
	task := &libapi.Task{
		ID:          run.TaskRevision.Def.GetSlug(),
		Name:        run.TaskRevision.Def.GetName(),
		Slug:        run.TaskRevision.Def.GetSlug(),
		Description: run.TaskRevision.Def.GetDescription(),
		Parameters:  utr.Parameters,
	}

	return GetRunResponse{
		Run:  run,
		Task: task,
	}, nil
}

type CancelRunRequest struct {
	RunID string `json:"runID"`
}

func CancelRunHandler(ctx context.Context, state *state.State, r *http.Request, req CancelRunRequest) (struct{}, error) {
	_, err := state.UpdateRun(req.RunID, func(run *dev.LocalRun) error {
		if run.Status.IsTerminal() {
			return errors.Errorf("cannot cancel run %s (state is already terminal)", run.RunID)
		}
		run.CancelFn()
		run.Status = api.RunCancelled
		cancelTime := time.Now()
		run.CancelledAt = &cancelTime
		run.CancelledBy = run.CreatorID
		return nil
	})
	return struct{}{}, err
}

type ListRunsResponse struct {
	Runs []dev.LocalRun `json:"runs"`
}

func ListRunsHandler(ctx context.Context, state *state.State, r *http.Request) (ListRunsResponse, error) {
	taskSlug := r.URL.Query().Get("taskSlug")
	runs, err := state.GetRunHistory(ctx, taskSlug)
	if err != nil {
		return ListRunsResponse{}, err
	}

	return ListRunsResponse{
		Runs: runs,
	}, nil
}

// GetTaskInfoHandler handles requests to the /i/tasks/get?slug=<task_slug> endpoint.
func GetTaskInfoHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.Task, error) {
	taskSlug := r.URL.Query().Get("slug")
	if taskSlug == "" {
		return libapi.Task{}, libhttp.NewErrBadRequest("task slug was not supplied")
	}
	taskConfig, ok := state.TaskConfigs.Get(taskSlug)
	if !ok {
		return libapi.Task{}, libhttp.NewErrNotFound("task with slug %q not found", taskSlug)
	}

	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	metadata, err := state.GetTaskErrors(ctx, taskSlug, pointers.ToString(envSlug))
	if err != nil {
		return libapi.Task{}, err
	}
	// For our purposes, the libapi.Task and libapi.UpdateTaskRequest structs contain the same critical data.
	// Using UpdateTaskRequest and taskConfig.Def.GetUpdateTaskRequest() conveniently
	//  populates the needed fields (params, config attachments, etc.).
	// We don't use GetUpdateTaskRequest() directly here since it does additional validation and
	// we want to best-effort support invalid task definitions (e.g. unknown resources) so that we can render
	// corresponding validation errors in the UI.
	req := libapi.Task{
		Slug:         taskConfig.Def.GetSlug(),
		Name:         taskConfig.Def.GetName(),
		Description:  taskConfig.Def.GetDescription(),
		Runtime:      taskConfig.Def.GetRuntime(),
		Resources:    map[string]string{},
		UpdatedAt:    metadata.RefreshedAt,
		ExecuteRules: libapi.ExecuteRules{RestrictCallers: taskConfig.Def.RestrictCallers},
	}
	if resourceAttachments, err := taskConfig.Def.GetResourceAttachments(); err != nil {
		return libapi.Task{}, errors.Wrap(err, "getting resource attachments")
	} else if resourceAttachments != nil {
		req.Resources = resourceAttachments
	}
	configs, err := taskConfig.Def.GetConfigAttachments()
	if err != nil {
		return libapi.Task{}, errors.Wrap(err, "getting config attachments")
	}
	req.Configs = configs
	parameters, err := taskConfig.Def.GetParameters()
	if err != nil {
		return libapi.Task{}, errors.Wrap(err, "getting parameters")
	}
	req.Parameters = parameters
	kind, options, err := taskConfig.Def.GetKindAndOptions()
	if err != nil {
		return libapi.Task{}, errors.Wrap(err, "getting kind and options")
	}
	req.Kind = kind
	req.KindOptions = options
	return req, nil
}

type ViewInfo struct {
	libapi.View
	ViewsPkgVersion string `json:"viewsPkgVersion"`
}

// GetViewInfoHandler handles requests to the /i/views/get?slug=<view_slug> endpoint.
func GetViewInfoHandler(ctx context.Context, state *state.State, r *http.Request) (ViewInfo, error) {
	viewSlug := r.URL.Query().Get("slug")
	if viewSlug == "" {
		return ViewInfo{}, libhttp.NewErrBadRequest("view slug was not supplied")
	}
	viewConfig, ok := state.ViewConfigs.Get(viewSlug)
	if !ok {
		return ViewInfo{}, libhttp.NewErrBadRequest("view with slug %q not found", viewSlug)
	}

	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	configVars := state.DevConfig.ConfigVars
	if len(state.DevConfig.EnvVars) > 0 || len(viewConfig.Def.EnvVars) > 0 {
		var err error
		configVars, err = configs.MergeRemoteConfigs(ctx, state, envSlug)
		if err != nil {
			return ViewInfo{}, errors.Wrap(err, "merging local and remote configs")
		}
	}

	apiPort := state.LocalClient.AppURL().Port()
	viewURL := utils.StudioURL(state.StudioURL.Host, apiPort, "/view/"+viewConfig.Def.Slug)

	headers := map[string]string{}
	if envSlug == nil {
		headers["X-Airplane-Studio-Fallback-Env-Slug"] = serverutils.NO_FALLBACK_ENVIRONMENT
	} else {
		headers["X-Airplane-Studio-Fallback-Env-Slug"] = pointers.ToString(envSlug)
	}

	if state.SandboxState != nil && state.DevToken != nil {
		headers["X-Airplane-Sandbox-Token"] = *state.DevToken
	}

	envVars, err := dev.GetEnvVarsForView(ctx, state.RemoteClient, dev.GetEnvVarsForViewConfig{
		ViewEnvVars:      viewConfig.Def.EnvVars,
		DevConfigEnvVars: state.DevConfig.EnvVars,
		ConfigVars:       configVars,
		FallbackEnvSlug:  pointers.ToString(envSlug),
		AuthInfo:         state.AuthInfo,
		Name:             viewConfig.Def.Name,
		Slug:             viewConfig.Def.Slug,
		ViewURL:          viewURL,
		APIHeaders:       headers,
	})
	if err != nil {
		return ViewInfo{}, errors.Wrap(err, "getting env vars for view")
	}

	// Try to read the @airplane/views version.
	viewsVersion := ""
	rootPackageJSON := filepath.Join(viewConfig.Root, "package.json")
	hasPackageJSON := fsx.AssertExistsAll(rootPackageJSON) == nil
	if hasPackageJSON {
		pkg, err := build.ReadPackageJSON(rootPackageJSON)
		if err != nil {
			return ViewInfo{}, errors.Wrap(err, "reading package.json")
		}
		viewsVersion = pkg.Dependencies["@airplane/views"]
	}

	return ViewInfo{
		View: libapi.View{
			ID:          viewConfig.Def.Slug,
			Slug:        viewConfig.Def.Slug,
			Name:        viewConfig.Def.Name,
			Description: viewConfig.Def.Description,
			EnvVars:     envVars,
		},
		ViewsPkgVersion: viewsVersion,
	}, nil
}

type GetConfigResponse struct {
	Config env.ConfigWithEnv `json:"config"`
}

func GetConfigHandler(ctx context.Context, state *state.State, r *http.Request) (GetConfigResponse, error) {
	id := r.URL.Query().Get("id")
	if id == "" {
		return GetConfigResponse{}, libhttp.NewErrBadRequest("id cannot be empty")
	}

	for _, c := range state.DevConfig.ConfigVars {
		if c.ID == id {
			return GetConfigResponse{
				Config: c,
			}, nil
		}
	}

	return GetConfigResponse{}, libhttp.NewErrBadRequest("config with id %q not found", id)
}

type UpsertConfigRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func UpsertConfigHandler(ctx context.Context, state *state.State, r *http.Request, req UpsertConfigRequest) (struct{}, error) {
	if err := state.DevConfig.SetConfigVar(req.Name, req.Value); err != nil {
		return struct{}{}, errors.Wrap(err, "setting config var")
	}

	return struct{}{}, nil
}

type DeleteConfigRequest struct {
	ID string `json:"configID"`
}

func DeleteConfigHandler(ctx context.Context, state *state.State, r *http.Request, req DeleteConfigRequest) (struct{}, error) {
	for _, c := range state.DevConfig.ConfigVars {
		if c.ID == req.ID {
			if err := state.DevConfig.DeleteConfigVar(c.Name); err != nil {
				return struct{}{}, errors.Wrap(err, "deleting config var")
			}
			return struct{}{}, nil
		}
	}

	return struct{}{}, libhttp.NewErrNotFound("config not found: %s", req.ID)
}

type ListConfigsResponse struct {
	Configs []env.ConfigWithEnv `json:"configs"`
}

func ListConfigsHandler(ctx context.Context, state *state.State, r *http.Request) (ListConfigsResponse, error) {
	configsWithEnv := maps.Values(state.DevConfig.ConfigVars)
	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)

	// Append any remote configs, if a fallback environment is set
	if envSlug != nil {
		remoteConfigs, err := configs.ListRemoteConfigs(ctx, state, *envSlug)
		if err == nil {
			remoteEnv, err := state.GetEnv(ctx, *envSlug)
			if err != nil {
				return ListConfigsResponse{}, err
			}
			for _, cfg := range remoteConfigs {
				configsWithEnv = append(configsWithEnv, env.ConfigWithEnv{
					Config: cfg,
					Remote: true,
					Env:    remoteEnv,
				})
			}
		} else {
			logger.Error("error fetching remote configs: %v", err)
		}
	}

	return ListConfigsResponse{
		Configs: configsWithEnv,
	}, nil
}

func CreateUploadHandler(
	ctx context.Context,
	state *state.State,
	r *http.Request,
	req libapi.CreateUploadRequest,
) (libapi.CreateUploadResponse, error) {
	resp, err := state.RemoteClient.CreateUpload(ctx, req)
	if err != nil {
		return libapi.CreateUploadResponse{}, errors.Wrap(err, "creating upload")
	}

	return libapi.CreateUploadResponse{
		Upload:       resp.Upload,
		ReadOnlyURL:  resp.ReadOnlyURL,
		WriteOnlyURL: resp.WriteOnlyURL,
	}, nil
}

func GetUploadHandler(
	ctx context.Context,
	state *state.State,
	r *http.Request,
	req libapi.GetUploadRequest,
) (libapi.GetUploadResponse, error) {
	resp, err := state.RemoteClient.GetUpload(ctx, req.UploadID)
	if err != nil {
		return libapi.GetUploadResponse{}, errors.Wrap(err, "getting upload")
	}

	return libapi.GetUploadResponse{
		Upload:      resp.Upload,
		ReadOnlyURL: resp.ReadOnlyURL,
	}, nil
}

type ListEnvsResponse struct {
	Envs []libapi.Env `json:"envs"`
}

func ListEnvsHandler(ctx context.Context, state *state.State, r *http.Request) (ListEnvsResponse, error) {
	resp, err := state.RemoteClient.ListEnvs(ctx)
	if err != nil {
		return ListEnvsResponse{}, errors.Wrap(err, "error getting envs")
	}

	envs := map[string]libapi.Env{}
	for _, env := range resp.Envs {
		envs[env.Slug] = env
	}
	state.EnvCache.ReplaceItems(envs)
	return ListEnvsResponse{
		Envs: resp.Envs,
	}, nil
}
