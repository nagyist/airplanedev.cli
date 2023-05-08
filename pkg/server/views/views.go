package views

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	libapi "github.com/airplanedev/cli/pkg/api"
	libhttp "github.com/airplanedev/cli/pkg/api/http"
	"github.com/airplanedev/cli/pkg/build/node"
	"github.com/airplanedev/cli/pkg/configs"
	"github.com/airplanedev/cli/pkg/definitions/updaters"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/server/state"
	serverutils "github.com/airplanedev/cli/pkg/server/utils"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

type ViewInfo struct {
	libapi.View
	ViewsPkgVersion string `json:"viewsPkgVersion"`
	// File is the (absolute) path to the file where this view is defined.
	File string `json:"file"`
}

// GetViewHandler handles requests to the /i/views/get?slug=<view_slug> endpoint.
func GetViewHandler(ctx context.Context, state *state.State, r *http.Request) (ViewInfo, error) {
	viewSlug := r.URL.Query().Get("slug")
	if viewSlug == "" {
		return ViewInfo{}, libhttp.NewErrBadRequest("view slug was not supplied")
	}
	view, ok := state.LocalViews.Get(viewSlug)
	if !ok {
		return ViewInfo{}, libhttp.NewErrBadRequest("view with slug %q not found", viewSlug)
	}

	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	configVars := state.DevConfig.ConfigVars
	if len(state.DevConfig.EnvVars) > 0 || len(view.Def.EnvVars) > 0 {
		var err error
		configVars, err = configs.MergeRemoteConfigs(ctx, state, envSlug)
		if err != nil {
			return ViewInfo{}, errors.Wrap(err, "merging local and remote configs")
		}
	}

	apiPort := state.LocalClient.AppURL().Port()
	viewURL := utils.StudioURL(state.StudioURL.Host, apiPort, "/view/"+view.Def.Slug)

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
		ViewEnvVars:      view.Def.EnvVars,
		DevConfigEnvVars: state.DevConfig.EnvVars,
		ConfigVars:       configVars,
		FallbackEnvSlug:  pointers.ToString(envSlug),
		AuthInfo:         state.AuthInfo,
		Name:             view.Def.Name,
		Slug:             view.Def.Slug,
		ViewURL:          viewURL,
		APIHeaders:       headers,
	})
	if err != nil {
		return ViewInfo{}, errors.Wrap(err, "getting env vars for view")
	}

	// Try to read the @airplane/views version.
	viewsVersion := ""
	rootPackageJSON := filepath.Join(view.Root, "package.json")
	hasPackageJSON := fsx.AssertExistsAll(rootPackageJSON) == nil
	if hasPackageJSON {
		pkg, err := node.ReadPackageJSON(rootPackageJSON)
		if err != nil {
			return ViewInfo{}, errors.Wrap(err, "reading package.json")
		}
		viewsVersion = pkg.Dependencies["@airplane/views"]
	}

	return viewStateToInfo(view, envVars, &viewsVersion), nil
}

func UpdateViewHandler(ctx context.Context, s *state.State, r *http.Request, req libapi.UpdateViewRequest) (struct{}, error) {
	viewConfig, ok := s.LocalViews.Get(req.Slug)
	if !ok {
		return struct{}{}, libhttp.NewErrNotFound("view with slug %q not found", req.Slug)
	}

	if err := viewConfig.Def.Update(req); err != nil {
		return struct{}{}, libhttp.NewErrBadRequest("unable to update view %q: %s", req.Slug, err.Error())
	}

	// Update the underlying view file.
	if err := updaters.UpdateView(ctx, s.Logger, viewConfig.Def.DefnFilePath, req.Slug, viewConfig.Def); err != nil {
		return struct{}{}, err
	}

	// Optimistically update the view in the cache.
	_, err := s.LocalViews.Update(req.Slug, func(val *state.ViewState) error {
		val.Def = viewConfig.Def
		val.UpdatedAt = time.Now()
		return nil
	})
	if err != nil {
		return struct{}{}, err
	}

	return struct{}{}, nil
}

type CanUpdateViewRequest struct {
	Slug string `json:"slug"`
}

type CanUpdateViewResponse struct {
	CanUpdate bool `json:"canUpdate"`
}

func CanUpdateViewHandler(ctx context.Context, state *state.State, r *http.Request) (CanUpdateViewResponse, error) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		return CanUpdateViewResponse{}, libhttp.NewErrBadRequest("view slug was not supplied")
	}

	viewConfig, ok := state.LocalViews.Get(slug)
	if !ok {
		return CanUpdateViewResponse{}, libhttp.NewErrNotFound("view with slug %q not found", slug)
	}

	canUpdate, err := updaters.CanUpdateView(ctx, state.Logger, viewConfig.Def.DefnFilePath, slug)
	if err != nil {
		return CanUpdateViewResponse{}, err
	}

	return CanUpdateViewResponse{
		CanUpdate: canUpdate,
	}, nil
}

type ListViewsResponse struct {
	Views []ViewInfo `json:"views"`
}

func ListViewsHandler(ctx context.Context, state *state.State, r *http.Request) (ListViewsResponse, error) {
	views := ListViews(state)

	return ListViewsResponse{
		Views: views,
	}, nil
}

func ListViews(state *state.State) []ViewInfo {
	viewStates := state.LocalViews.Values()
	views := make([]ViewInfo, 0, len(viewStates))

	for _, viewState := range viewStates {
		views = append(views, viewStateToInfo(viewState, nil, nil))
	}

	slices.SortFunc(views, func(a, b ViewInfo) bool {
		return a.Slug < b.Slug
	})

	return views
}

func viewStateToInfo(viewState state.ViewState, envVars map[string]string, viewsPkgVersion *string) ViewInfo {
	vi := ViewInfo{
		View: libapi.View{
			ID:              viewState.Def.Slug,
			Slug:            viewState.Def.Slug,
			Name:            viewState.Def.Name,
			Description:     viewState.Def.Description,
			EnvVars:         viewState.Def.EnvVars,
			ResolvedEnvVars: envVars,
			UpdatedAt:       viewState.UpdatedAt,
		},
	}

	if viewsPkgVersion != nil {
		vi.ViewsPkgVersion = *viewsPkgVersion
	}

	vi.File = viewState.Def.DefnFilePath

	return vi
}
