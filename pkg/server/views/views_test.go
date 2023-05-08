package views

import (
	"testing"

	libapi "github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/stretchr/testify/require"
)

func TestListViews(t *testing.T) {
	require := require.New(t)

	s := &state.State{
		LocalViews: state.NewStore(map[string]state.ViewState{
			"view_1": {
				ViewConfig: discover.ViewConfig{
					Def: definitions.ViewDefinition{
						Name:        "My view",
						Entrypoint:  "my_view.ts",
						Slug:        "view_1",
						Description: "My awesome view",
					},
					Source: discover.ConfigSourceDefn,
				},
			},
			"view_2": {
				ViewConfig: discover.ViewConfig{
					Def: definitions.ViewDefinition{
						Name:        "My view 2",
						Entrypoint:  "my_view_2.ts",
						Slug:        "view_2",
						Description: "My awesome view 2",
					},
					Source: discover.ConfigSourceDefn,
				},
			},
		}),
	}

	require.ElementsMatch([]ViewInfo{
		{
			View: libapi.View{
				ID:          "view_1",
				Slug:        "view_1",
				Name:        "My view",
				Description: "My awesome view",
			},
		},
		{
			View: libapi.View{
				ID:          "view_2",
				Slug:        "view_2",
				Name:        "My view 2",
				Description: "My awesome view 2",
			},
		},
	}, ListViews(s))
}

func TestViewConfigToInfo(t *testing.T) {
	require := require.New(t)

	viewSlug := "my_view"
	viewDefinition := definitions.ViewDefinition{
		Name:        "My view",
		Entrypoint:  "my_view.ts",
		Slug:        viewSlug,
		Description: "My awesome view",
	}
	viewState := state.ViewState{
		ViewConfig: discover.ViewConfig{
			Def:    viewDefinition,
			Source: discover.ConfigSourceDefn,
		},
	}

	viewsVersion := "2.0.0"
	viewEnvVars := map[string]string{
		"KEY": "VALUE",
	}
	view := viewStateToInfo(viewState, viewEnvVars, &viewsVersion)

	require.Equal(ViewInfo{
		View: libapi.View{
			ID:              viewSlug,
			Slug:            viewSlug,
			Name:            "My view",
			Description:     "My awesome view",
			ResolvedEnvVars: viewEnvVars,
		},
		ViewsPkgVersion: viewsVersion,
	}, view)
}
