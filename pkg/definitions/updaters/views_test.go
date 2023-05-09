package updaters

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/stretchr/testify/require"
)

func TestUpdateView(t *testing.T) {
	testCases := []struct {
		name string
		slug string
		def  definitions.ViewDefinition
		ext  string
	}{
		{
			// Tests setting various fields.
			name: "all",
			slug: "my_view",
			def: definitions.ViewDefinition{
				// This case also tests renaming a slug.
				Slug:        "my_view_2",
				Name:        "View name",
				Description: "View description",
				EnvVars: api.EnvVars{
					"CONFIG": api.EnvVarValue{
						Config: pointers.String("aws_access_key"),
					},
					"VALUE": api.EnvVarValue{
						Value: pointers.String("Hello World!"),
					},
				},
			},
		},
		{
			// Tests the case where values are cleared.
			name: "cleared",
			slug: "my_view",
			def: definitions.ViewDefinition{
				Slug: "my_view",
			},
		},
		{
			// Tests the case where values are set to their default values (and therefore should not be serialized).
			name: "all_defaults",
			slug: "my_view",
			def: definitions.ViewDefinition{
				Slug:        "my_view",
				Name:        "",
				Description: "",
				EnvVars:     api.EnvVars{},
			},
		},
		{
			// Tests the case a file contains tasks and multiple views. Only the View with matching slug
			// should be updated.
			name: "multiple_entities",
			slug: "my_entity_2",
			def: definitions.ViewDefinition{
				Slug: "my_entity_two",
				Name: "My entity (v2)",
			},
		},
		{
			// Tests the case where the file contains invalid JSX.
			name: "invalid_code",
			slug: "my_view",
			def: definitions.ViewDefinition{
				Slug:        "my_view",
				Description: "Added a description!",
			},
		},
		{
			// Tests the case where the file contains TSX.
			name: "typescript",
			slug: "my_view",
			def: definitions.ViewDefinition{
				Slug:        "my_view",
				Description: "Added a description!",
			},
			ext: ".airplane.tsx",
		},
		{
			// Tests managing a View as YAML.
			name: "yaml",
			slug: "my_view",
			def: definitions.ViewDefinition{
				Slug:        "my_view_2",
				Description: "Added a description!",
				Entrypoint:  "./MyView.jsx",
			},
			ext: ".view.yaml",
		},
		{
			// Tests managing a View as JSON.
			name: "json",
			slug: "my_view",
			def: definitions.ViewDefinition{
				Slug:        "my_view_2",
				Description: "Added a description!",
				Entrypoint:  "./MyView.jsx",
			},
			ext: ".view.json",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			require := require.New(t)
			l := logger.NewTestLogger(t)

			// Clone the input file into a temporary directory as it will be overwritten by `Update()`.
			if tC.ext == "" {
				tC.ext = ".airplane.jsx"
			}
			in, err := os.Open(fmt.Sprintf("./view/fixtures/%s", tC.name+tC.ext))
			require.NoError(err)
			f, err := os.CreateTemp("", "runtime-update-javascript-*"+tC.ext)
			require.NoError(err)
			t.Cleanup(func() {
				require.NoError(os.Remove(f.Name()))
			})
			_, err = io.Copy(f, in)
			require.NoError(err)
			require.NoError(f.Close())

			ok, err := CanUpdateView(context.Background(), l, f.Name(), tC.slug)
			require.NoError(err)
			require.True(ok)

			// Perform the update on the temporary file.
			err = UpdateView(context.Background(), l, f.Name(), tC.slug, tC.def)
			require.NoError(err)

			// Compare
			actual, err := os.ReadFile(f.Name())
			require.NoError(err)
			expected, err := os.ReadFile(fmt.Sprintf("./view/fixtures/%s.out%s", tC.name, tC.ext))
			require.NoError(err)
			require.Equal(string(expected), string(actual))
		})
	}
}

func TestCanUpdateView(t *testing.T) {
	testCases := []struct {
		slug      string
		canUpdate bool
	}{
		{
			slug:      "spread",
			canUpdate: false,
		},
		{
			slug:      "computed",
			canUpdate: false,
		},
		{
			slug:      "key",
			canUpdate: false,
		},
		{
			slug:      "template",
			canUpdate: false,
		},
		{
			slug:      "tagged_template",
			canUpdate: false,
		},
		{
			// There is no view that matches this slug.
			slug:      "slug_not_found",
			canUpdate: false,
		},
		{
			slug:      "good",
			canUpdate: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.slug, func(t *testing.T) {
			require := require.New(t)
			l := logger.NewTestLogger(t)

			canUpdate, err := CanUpdateView(context.Background(), l, "./view/fixtures/can_update.airplane.jsx", tC.slug)
			require.NoError(err)
			require.Equal(tC.canUpdate, canUpdate)
		})
	}
}
