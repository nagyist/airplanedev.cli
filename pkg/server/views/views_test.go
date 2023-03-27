package views

import (
	"testing"

	"github.com/airplanedev/cli/pkg/server/state"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/stretchr/testify/require"
)

func TestListViews(t *testing.T) {
	require := require.New(t)

	s := &state.State{
		ViewConfigs: state.NewStore[string, discover.ViewConfig](map[string]discover.ViewConfig{
			"view_1": {
				Def: definitions.ViewDefinition{
					Name:        "My view",
					Entrypoint:  "my_view.ts",
					Slug:        "view_1",
					Description: "My awesome view",
				},
				Source: discover.ConfigSourceDefn,
			},
			"view_2": {
				Def: definitions.ViewDefinition{
					Name:        "My view 2",
					Entrypoint:  "my_view_2.ts",
					Slug:        "view_2",
					Description: "My awesome view 2",
				},
				Source: discover.ConfigSourceDefn,
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
	viewConfig := discover.ViewConfig{
		Def:    viewDefinition,
		Source: discover.ConfigSourceDefn,
	}

	viewsVersion := "2.0.0"
	viewEnvVars := map[string]string{
		"KEY": "VALUE",
	}
	view := viewConfigToInfo(viewConfig, viewEnvVars, &viewsVersion)

	require.Equal(ViewInfo{
		View: libapi.View{
			ID:          viewSlug,
			Slug:        viewSlug,
			Name:        "My view",
			Description: "My awesome view",
			EnvVars:     viewEnvVars,
		},
		ViewsPkgVersion: viewsVersion,
	}, view)
}
