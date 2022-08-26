package server

import (
	"context"
	"fmt"
	"net/http"

	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/conversion"
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// AttachInternalAPIRoutes attaches a minimal subset of the internal Airplane API endpoints that are necessary for the
// previewer
func AttachInternalAPIRoutes(r *mux.Router, state *State) {
	const basePath = "/i/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/resources/create", HandlerWithBody(state, CreateResourceHandler)).Methods("POST", "OPTIONS")
	r.Handle("/resources/get", Handler(state, GetResourceHandler)).Methods("GET", "OPTIONS")
	r.Handle("/resources/list", Handler(state, ListResourcesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/resources/update", HandlerWithBody(state, UpdateResourceHandler)).Methods("POST", "OPTIONS")

	r.Handle("/runs/getDescendants", Handler(state, GetDescendantsHandler)).Methods("GET", "OPTIONS")
}

type CreateResourceRequest struct {
	Name       string                          `json:"name"`
	Slug       string                          `json:"slug"`
	Kind       resources.ResourceKind          `json:"kind"`
	KindConfig kind_configs.ResourceKindConfig `json:"kindConfig"`
}

type CreateResourceResponse struct {
	ResourceID string `json:"resourceID"`
}

// CreateResourceHandler handles requests to the /v0/resources/get endpoint
func CreateResourceHandler(ctx context.Context, state *State, r *http.Request, req CreateResourceRequest) (CreateResourceResponse, error) {
	internalResource := kind_configs.InternalResource{
		Slug:       req.Slug,
		Kind:       req.Kind,
		Name:       req.Name,
		KindConfig: req.KindConfig,
	}

	resource, err := internalResource.ToExternalResource()
	if err != nil {
		return CreateResourceResponse{}, errors.Wrap(err, "converting to external resource")
	}

	if err := state.devConfig.SetResource(req.Slug, resource); err != nil {
		return CreateResourceResponse{}, errors.Wrap(err, "setting resource")
	}

	return CreateResourceResponse{ResourceID: fmt.Sprintf("res-%s", req.Slug)}, nil
}

type GetResourceResponse struct {
	Resource kind_configs.InternalResource
}

// GetResourceHandler handles requests to the /v0/resources/get endpoint
func GetResourceHandler(ctx context.Context, state *State, r *http.Request) (GetResourceResponse, error) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		return GetResourceResponse{}, errors.Errorf("Resource slug was not supplied")
	}

	for s, resource := range state.devConfig.Resources {
		if s == slug {
			internalResource, err := conversion.ConvertToInternalResource(resource, state.logger)
			if err != nil {
				return GetResourceResponse{}, errors.Wrap(err, "converting to internal resource")
			}
			return GetResourceResponse{Resource: internalResource}, nil
		}
	}

	return GetResourceResponse{}, errors.Errorf("Resource with slug %s is not in dev config file", slug)
}

// ListResourcesHandler handles requests to the /v0/resources/list endpoint
func ListResourcesHandler(ctx context.Context, state *State, r *http.Request) (libapi.ListResourcesResponse, error) {
	resources := make([]libapi.Resource, 0, len(state.devConfig.RawResources))
	for slug, resource := range state.devConfig.Resources {
		// It doesn't matter what we include in the resource struct, as long as we include the slug - this handler
		// is only used so that requests to the local dev api server for this endpoint don't error, in particular:
		// https://github.com/airplanedev/lib/blob/d4c8ed7d1b30095c5cacac2b5c4da8f3ada6378f/pkg/deploy/taskdir/definitions/def_0_3.go#L1081-L1087
		resources = append(resources, libapi.Resource{
			Slug: slug,
			Kind: libapi.ResourceKind(resource.Kind()),
		})
	}

	return libapi.ListResourcesResponse{
		Resources: resources,
	}, nil
}

type UpdateResourceRequest struct {
	ID         string                          `json:"id"`
	Slug       string                          `json:"slug"`
	Name       string                          `json:"name"`
	Kind       resources.ResourceKind          `json:"kind"`
	KindConfig kind_configs.ResourceKindConfig `json:"kindConfig"`
}

type UpdateResourceResponse struct {
	ResourceID string `json:"resourceID"`
}

// UpdateResourceHandler handles requests to the /v0/resources/get endpoint
func UpdateResourceHandler(ctx context.Context, state *State, r *http.Request, req UpdateResourceRequest) (UpdateResourceResponse, error) {
	resource, ok := state.devConfig.Resources[req.Slug]
	if !ok {
		return UpdateResourceResponse{}, errors.Errorf("resource with slug %s not found in dev config file", req.Slug)
	}

	// Convert to internal representation of resource for updating.
	internalResource, err := conversion.ConvertToInternalResource(resource, state.logger)
	if err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "converting to external resource")
	}

	// Update internal resource - utilize KindConfig.Update to not overwrite sensitive fields.
	internalResource.Slug = req.Slug
	internalResource.Name = req.Name
	if err := internalResource.KindConfig.Update(req.KindConfig); err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "updating kind config of internal resource")
	}

	// Convert back to external representation of resource.
	newResource, err := internalResource.ToExternalResource()
	if err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "converting to external resource")
	}

	if err := state.devConfig.SetResource(req.Slug, newResource); err != nil {
		return UpdateResourceResponse{}, errors.Wrap(err, "setting resource")
	}

	return UpdateResourceResponse{
		ResourceID: req.Slug,
	}, nil
}

type GetDescendantsResponse struct {
	Descendants []LocalRun `json:"descendants"`
}

func GetDescendantsHandler(ctx context.Context, state *State, r *http.Request) (GetDescendantsResponse, error) {
	runID := r.URL.Query().Get("runID")
	if runID == "" {
		return GetDescendantsResponse{}, errors.New("runID cannot be empty")
	}
	descendants := state.runs.getDescendants(runID)

	return GetDescendantsResponse{
		Descendants: descendants,
	}, nil
}
