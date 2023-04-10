package views

import (
	"context"
	"net/http"
	"path/filepath"

	libapi "github.com/airplanedev/cli/pkg/api"
	libhttp "github.com/airplanedev/cli/pkg/api/http"
	"github.com/airplanedev/cli/pkg/build/node"
	"github.com/airplanedev/cli/pkg/configs"
	"github.com/airplanedev/cli/pkg/deploy/discover"
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
}

// GetViewHandler handles requests to the /i/views/get?slug=<view_slug> endpoint.
func GetViewHandler(ctx context.Context, state *state.State, r *http.Request) (ViewInfo, error) {
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
		pkg, err := node.ReadPackageJSON(rootPackageJSON)
		if err != nil {
			return ViewInfo{}, errors.Wrap(err, "reading package.json")
		}
		viewsVersion = pkg.Dependencies["@airplane/views"]
	}

	return viewConfigToInfo(viewConfig, envVars, &viewsVersion), nil
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
	viewConfigs := state.ViewConfigs.Values()
	views := make([]ViewInfo, 0, len(viewConfigs))

	for _, vc := range viewConfigs {
		views = append(views, viewConfigToInfo(vc, nil, nil))
	}

	slices.SortFunc(views, func(a, b ViewInfo) bool {
		return a.Slug < b.Slug
	})

	return views
}

func viewConfigToInfo(viewConfig discover.ViewConfig, envVars map[string]string, viewsPkgVersion *string) ViewInfo {
	vi := ViewInfo{
		View: libapi.View{
			ID:          viewConfig.Def.Slug,
			Slug:        viewConfig.Def.Slug,
			Name:        viewConfig.Def.Name,
			Description: viewConfig.Def.Description,
			EnvVars:     envVars,
		},
	}

	if viewsPkgVersion != nil {
		vi.ViewsPkgVersion = *viewsPkgVersion
	}

	return vi
}
