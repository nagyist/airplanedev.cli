package apiint

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/logger"
	res "github.com/airplanedev/cli/pkg/resource"
	"github.com/airplanedev/cli/pkg/server/handlers"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/conversion"
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/gorilla/mux"
	"github.com/gosimple/slug"
	"github.com/pkg/errors"
)

// AttachInternalAPIRoutes attaches a minimal subset of the internal Airplane API endpoints that are necessary for the
// previewer
func AttachInternalAPIRoutes(r *mux.Router, state *state.State) {
	const basePath = "/i/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/resources/create", handlers.HandlerWithBody(state, CreateResourceHandler)).Methods("POST", "OPTIONS")
	r.Handle("/resources/get", handlers.Handler(state, GetResourceHandler)).Methods("GET", "OPTIONS")
	r.Handle("/resources/list", handlers.Handler(state, ListResourcesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/resources/update", handlers.HandlerWithBody(state, UpdateResourceHandler)).Methods("POST", "OPTIONS")
	r.Handle("/resources/delete", handlers.HandlerWithBody(state, DeleteResourceHandler)).Methods("POST", "OPTIONS")
	r.Handle("/resources/isSlugAvailable", handlers.Handler(state, IsResourceSlugAvailableHandler)).Methods("GET", "OPTIONS")

	r.Handle("/displays/list", handlers.Handler(state, ListDisplaysHandler)).Methods("GET", "OPTIONS")

	r.Handle("/prompts/list", handlers.Handler(state, ListPromptHandler)).Methods("GET", "OPTIONS")
	r.Handle("/prompts/submit", handlers.HandlerWithBody(state, SubmitPromptHandler)).Methods("POST", "OPTIONS")

	r.Handle("/runs/get", handlers.Handler(state, GetRunHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/getDescendants", handlers.Handler(state, GetDescendantsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/list", handlers.Handler(state, ListRunsHandler)).Methods("GET", "OPTIONS")

	r.Handle("/tasks/get", handlers.Handler(state, GetTaskInfoHandler)).Methods("GET", "OPTIONS")

	r.Handle("/users/get", handlers.Handler(state, GetUserHandler)).Methods("GET", "OPTIONS")
}

type CreateResourceRequest struct {
	Name           string                 `json:"name"`
	Slug           string                 `json:"slug"`
	Kind           resources.ResourceKind `json:"kind"`
	ExportResource resources.Resource     `json:"resource"`
}

func (r *CreateResourceRequest) UnmarshalJSON(buf []byte) error {
	var raw struct {
		Name           string                 `json:"name"`
		Slug           string                 `json:"slug"`
		Kind           resources.ResourceKind `json:"kind"`
		ExportResource map[string]interface{} `json:"resource"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	var export resources.Resource
	var err error
	if raw.ExportResource != nil {
		export, err = resources.GetResource(resources.ResourceKind(raw.Kind), raw.ExportResource)
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
			return CreateResourceResponse{}, errors.New("Could not generate unique resource slug")
		}
	} else {
		if _, ok := state.DevConfig.Resources[resourceSlug]; ok {
			return CreateResourceResponse{}, errors.Errorf("Resource with slug %s already exists", resourceSlug)
		}
	}

	if err := req.ExportResource.Calculate(); err != nil {
		return CreateResourceResponse{}, errors.Wrap(err, "computing precalculated fields")
	}

	id := fmt.Sprintf("res-%s", resourceSlug)
	resource := req.ExportResource
	if err := resource.UpdateBaseResource(resources.BaseResource{
		ID:   id,
		Slug: resourceSlug,
		Kind: req.Kind,
		Name: req.Name,
	}); err != nil {
		return CreateResourceResponse{}, errors.Wrap(err, "updating base resoruce")
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
		return GetResourceResponse{}, errors.Errorf("Resource slug was not supplied")
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

	return GetResourceResponse{}, errors.Errorf("Resource with slug %s is not in dev config file", slug)
}

type APIResourceWithEnv struct {
	libapi.Resource
	Remote bool `json:"remote"`
}

type ListResourcesResponse struct {
	Resources []APIResourceWithEnv `json:"resources"`
}

// ListResourcesHandler handles requests to the /i/resources/list endpoint
func ListResourcesHandler(ctx context.Context, state *state.State, r *http.Request) (ListResourcesResponse, error) {
	resources := make([]APIResourceWithEnv, 0)
	for slug, r := range state.DevConfig.Resources {
		resources = append(resources, APIResourceWithEnv{
			Resource: libapi.Resource{
				ID:                r.Resource.ID(),
				Name:              slug, // TODO: Change to actual name of resource once that's exposed from export resource.
				Slug:              slug,
				Kind:              libapi.ResourceKind(r.Resource.Kind()),
				ExportResource:    r.Resource,
				CanUseResource:    true,
				CanUpdateResource: true,
			},
			Remote: false,
		})
	}

	if state.EnvID == env.LocalEnvID {
		remoteResources, err := res.ListRemoteResources(ctx, state)
		if err != nil {
			return ListResourcesResponse{}, errors.Wrap(err, "fetching remote resources")
		}

		for _, r := range remoteResources {
			// This is purely so we can display remote resource information in the local dev editor. The remote list
			// resources endpoint doesn't return CanUseResource or CanUpdateResource, and so we set them to true here.
			r.CanUseResource = true
			r.CanUpdateResource = true
			resources = append(resources, APIResourceWithEnv{
				Resource: r,
				Remote:   true,
			})
		}
	}

	return ListResourcesResponse{
		Resources: resources,
	}, nil
}

type UpdateResourceRequest struct {
	ID             string                 `json:"id"`
	Slug           string                 `json:"slug"`
	Name           string                 `json:"name"`
	Kind           resources.ResourceKind `json:"kind"`
	ExportResource resources.Resource     `json:"resource"`
}

func (r *UpdateResourceRequest) UnmarshalJSON(buf []byte) error {
	var raw struct {
		ID             string                 `json:"id"`
		Slug           string                 `json:"slug"`
		Name           string                 `json:"name"`
		Kind           resources.ResourceKind `json:"kind"`
		ExportResource map[string]interface{} `json:"resource"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	var export resources.Resource
	var err error
	if raw.ExportResource != nil {
		export, err = resources.GetResource(resources.ResourceKind(raw.Kind), raw.ExportResource)
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
	var resource resources.Resource
	for configSlug, configResource := range state.DevConfig.Resources {
		// We can't rely on the slug for existence since it may have changed.
		if configResource.Resource.ID() == req.ID {
			foundResource = true
			resource = configResource.Resource
			oldSlug = configSlug
			break
		}
	}
	if !foundResource {
		return UpdateResourceResponse{}, errors.Errorf("resource with slug %s not found in dev config file", req.Slug)
	}

	// Set a new resource ID based on the new slug.
	newResourceID := fmt.Sprintf("res-%s", req.Slug)

	if err := resource.Update(req.ExportResource); err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "updating resource")
	}
	if err := resource.UpdateBaseResource(resources.BaseResource{
		ID:   newResourceID,
		Slug: req.Slug,
		Name: req.Name,
	}); err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "updating base resoruce")
	}

	// Remove the old resource first - we need to do this since DevConfig.Resources is a mapping from slug to resource,
	// and if the update resource request involves updating the slug, we don't want to leave the old resource (under the
	// old slug) in the dev config file.
	if err := state.DevConfig.RemoveResource(oldSlug); err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "deleting old resource")
	}

	if err := state.DevConfig.SetResource(req.Slug, resource); err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "setting resource")
	}

	return UpdateResourceResponse{
		ResourceID: newResourceID,
	}, nil
}

type DeleteResourceRequest struct {
	ID string `json:"id"`
}

// DeleteResourceHandler handles requests to the /i/resources/delete endpoint
// The web app does utilize the response of the resource deletion handler.
func DeleteResourceHandler(ctx context.Context, state *state.State, r *http.Request, req DeleteResourceRequest) (struct{}, error) {
	id := req.ID
	resourceSlug, err := res.SlugFromID(id)
	if err != nil {
		return struct{}{}, err
	}

	if _, ok := state.DevConfig.Resources[resourceSlug]; !ok {
		return struct{}{}, errors.Errorf("resource with id %s does not exist in dev config file", id)
	}

	if err := state.DevConfig.RemoveResource(resourceSlug); err != nil {
		return struct{}{}, errors.Wrap(err, "setting resource")
	}

	return struct{}{}, nil
}

type IsResourceSlugAvailableResponse struct {
	Available bool `json:"available"`
}

// IsResourceSlugAvailableHandler handles requests to the /i/resources/isSlugAvailable endpoint
func IsResourceSlugAvailableHandler(ctx context.Context, state *state.State, r *http.Request) (IsResourceSlugAvailableResponse, error) {
	slug := r.URL.Query().Get("slug")
	configResource, ok := state.DevConfig.Resources[slug]
	return IsResourceSlugAvailableResponse{
		Available: !ok || configResource.Resource.ID() == r.URL.Query().Get("id"),
	}, nil
}

type ListDisplaysResponse struct {
	Displays []libapi.Display `json:"displays"`
}

func ListDisplaysHandler(ctx context.Context, state *state.State, r *http.Request) (ListDisplaysResponse, error) {
	runID := r.URL.Query().Get("runID")
	run, ok := state.Runs.Get(runID)
	if !ok {
		return ListDisplaysResponse{}, errors.Errorf("run with id %q not found", runID)
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
		return ListPromptsResponse{}, errors.New("runID is required")
	}

	run, ok := state.Runs.Get(runID)
	if !ok {
		return ListPromptsResponse{}, errors.New("run not found")
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
		return PromptResponse{}, errors.New("prompt ID is required")
	}
	if req.RunID == "" {
		return PromptResponse{}, errors.New("run ID is required")
	}

	userID := state.CliConfig.ParseTokenForAnalytics().UserID

	_, err := state.Runs.Update(req.RunID, func(run *dev.LocalRun) error {
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
		return errors.New("prompt does not exist")
	})
	if err != nil {
		return PromptResponse{}, err
	}
	return PromptResponse{ID: req.ID}, nil
}

type GetDescendantsResponse struct {
	Descendants []dev.LocalRun `json:"descendants"`
}

func GetDescendantsHandler(ctx context.Context, state *state.State, r *http.Request) (GetDescendantsResponse, error) {
	runID := r.URL.Query().Get("runID")
	if runID == "" {
		return GetDescendantsResponse{}, errors.New("runID cannot be empty")
	}
	descendants := state.Runs.GetDescendants(runID)

	return GetDescendantsResponse{
		Descendants: descendants,
	}, nil
}

type User struct {
	ID        string  `json:"userID" db:"id"`
	Email     string  `json:"email" db:"email"`
	Name      string  `json:"name" db:"name"`
	AvatarURL *string `json:"avatarURL" db:"avatar_url"`
}

type GetUserResponse struct {
	User User `json:"user"`
}

func GetUserHandler(ctx context.Context, state *state.State, r *http.Request) (GetUserResponse, error) {
	userID := r.URL.Query().Get("userID")
	// Set avatar to anonymous silhouette
	gravatarURL := "https://www.gravatar.com/avatar?d=mp"
	return GetUserResponse{
		User: User{
			ID:        userID,
			Email:     "hello@airplane.dev",
			Name:      "Airplane editor",
			AvatarURL: &gravatarURL,
		},
	}, nil
}

type GetRunResponse struct {
	Run  dev.LocalRun `json:"run"`
	Task *libapi.Task `json:"task"`
}

func GetRunHandler(ctx context.Context, state *state.State, r *http.Request) (GetRunResponse, error) {
	runID := r.URL.Query().Get("id")
	run, ok := state.Runs.Get(runID)
	if !ok {
		return GetRunResponse{}, errors.Errorf("run with id %s not found", runID)
	}
	response := GetRunResponse{Run: run}

	if taskConfig, ok := state.TaskConfigs[run.TaskID]; ok {
		utr, err := taskConfig.Def.GetUpdateTaskRequest(ctx, state.LocalClient)
		if err != nil {
			logger.Error("Encountered error while getting task info: %v", err)
			return GetRunResponse{}, errors.Errorf("error getting task %s", taskConfig.Def.GetSlug())
		}
		response.Task = &libapi.Task{
			ID:          taskConfig.Def.GetSlug(),
			Name:        taskConfig.Def.GetName(),
			Slug:        taskConfig.Def.GetSlug(),
			Description: taskConfig.Def.GetDescription(),
			Parameters:  utr.Parameters,
		}
	}

	return response, nil
}

type ListRunsResponse struct {
	Runs []dev.LocalRun `json:"runs"`
}

func ListRunsHandler(ctx context.Context, state *state.State, r *http.Request) (ListRunsResponse, error) {
	taskSlug := r.URL.Query().Get("taskSlug")
	runs := state.Runs.GetRunHistory(taskSlug)
	return ListRunsResponse{
		Runs: runs,
	}, nil
}

func ParamWithComponent(p libapi.Parameter) (libapi.Parameter, error) {
	switch p.Type {
	case "shorttext":
		p.Type = "string"
	case "longtext":
		p.Type = "string"
		p.Component = libapi.ComponentTextarea
	case "sql":
		p.Type = "string"
		p.Component = libapi.ComponentEditorSQL
	case "boolean", "upload", "integer", "float", "date", "datetime", "configvar":
		break
	default:
		return libapi.Parameter{}, errors.Errorf("unknown parameter type: %s", p.Type)
	}
	return p, nil
}

// GetTaskInfoHandler handles requests to the /i/tasks/get?slug=<task_slug> endpoint.
func GetTaskInfoHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.UpdateTaskRequest, error) {
	taskSlug := r.URL.Query().Get("slug")
	if taskSlug == "" {
		return libapi.UpdateTaskRequest{}, errors.New("Task slug was not supplied, request path must be of the form /v0/tasks?slug=<task_slug>")
	}
	taskConfig, ok := state.TaskConfigs[taskSlug]
	if !ok {
		return libapi.UpdateTaskRequest{}, errors.Errorf("Task with slug %s not found", taskSlug)
	}
	req := libapi.UpdateTaskRequest{
		Slug:        taskConfig.Def.GetSlug(),
		Name:        taskConfig.Def.GetName(),
		Description: taskConfig.Def.GetDescription(),
		Runtime:     taskConfig.Def.GetRuntime(),
		Resources:   map[string]string{},
	}
	if resources := taskConfig.Def.GetResourceAttachments(); resources != nil {
		req.Resources = resources
	}
	configs, err := taskConfig.Def.GetConfigAttachments()
	if err != nil {
		return libapi.UpdateTaskRequest{}, errors.Wrap(err, "getting config attachments")
	}
	req.Configs = &configs
	parameters := make(libapi.Parameters, len(taskConfig.Def.GetParameters()))
	for i, p := range taskConfig.Def.GetParameters() {
		p, err := ParamWithComponent(p)
		if err != nil {
			return libapi.UpdateTaskRequest{}, errors.Wrap(err, "getting parameters")
		}
		parameters[i] = p
	}
	req.Parameters = parameters
	kind, options, err := taskConfig.Def.GetKindAndOptions()
	if err != nil {
		return libapi.UpdateTaskRequest{}, errors.Wrap(err, "getting kind and options")
	}
	req.Kind = kind
	req.KindOptions = options
	return req, nil
}
