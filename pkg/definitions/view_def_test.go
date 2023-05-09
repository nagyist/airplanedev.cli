package definitions

import (
	"testing"

	"github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/stretchr/testify/require"
)

func TestViewDefinitionMarshal(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name                    string
		def                     ViewDefinition
		overrideUnmarshalledDef *ViewDefinition
		expectedYAML            string
		expectedJSON            string
	}{
		{
			name: "implicit defaults",
			def: ViewDefinition{
				Slug: "hello_world",
			},
			expectedYAML: `slug: hello_world
entrypoint: ""
`,
			expectedJSON: `{
	"slug": "hello_world",
	"entrypoint": ""
}
`,
		},
		{
			// This test sets all values explicitly to their defaults which should not be serialized.
			name: "explicit defaults",
			def: ViewDefinition{
				Slug:        "hello_world",
				Name:        "",
				Description: "",
				Entrypoint:  "",
				EnvVars:     api.EnvVars{},
			},
			// When unmarshalled, the default values will not be set.
			overrideUnmarshalledDef: &ViewDefinition{
				Slug: "hello_world",
			},
			expectedYAML: `slug: hello_world
entrypoint: ""
`,
			expectedJSON: `{
	"slug": "hello_world",
	"entrypoint": ""
}
`,
		},
		{
			// This test sets all values to non-default values. All should be serialized.
			name: "non-default values",
			def: ViewDefinition{
				Slug:        "hello_world",
				Name:        "Hello World",
				Description: "A starter view.",
				Entrypoint:  "entrypoint.tsx",
				EnvVars: api.EnvVars{
					"TEST_ENV": api.EnvVarValue{
						Value: pointers.String("hello world"),
					},
				},
			},
			expectedYAML: `slug: hello_world
name: Hello World
description: A starter view.
entrypoint: entrypoint.tsx
envVars:
  TEST_ENV:
    value: hello world
`,
			expectedJSON: `{
	"slug": "hello_world",
	"name": "Hello World",
	"description": "A starter view.",
	"entrypoint": "entrypoint.tsx",
	"envVars": {
		"TEST_ENV": {
			"value": "hello world"
		}
	}
}
`,
		},
		{
			name: "multiline strings",
			def: ViewDefinition{
				Slug:        "hello_world",
				Description: "This View does magic:\n\n  - Step 1: look fly\n  - Step 2: fly\n  - Step 3: ????\n  - Step 4: PROFIT!!!\n",
				Entrypoint:  "entrypoint.tsx",
			},
			// Use double quotes instead of backticks because this string contains two leading spaces that would get removed by a formatter.
			expectedYAML: "slug: hello_world\ndescription: |\n  This View does magic:\n  \n    - Step 1: look fly\n    - Step 2: fly\n    - Step 3: ????\n    - Step 4: PROFIT!!!\nentrypoint: entrypoint.tsx\n",
			expectedJSON: `{
	"slug": "hello_world",
	"description": "This View does magic:\n\n  - Step 1: look fly\n  - Step 2: fly\n  - Step 3: ????\n  - Step 4: PROFIT!!!\n",
	"entrypoint": "entrypoint.tsx"
}
`,
		},
	} {
		test := test // rebind for parallel tests
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require := require.New(t)

			for _, fmt := range []DefFormat{DefFormatYAML, DefFormatJSON} {
				expected := test.expectedYAML
				if fmt == DefFormatJSON {
					expected = test.expectedJSON
				}

				bytestr, err := test.def.Marshal(fmt)
				require.NoError(err)
				require.Equal(expected, string(bytestr))
				d := ViewDefinition{}
				err = d.Unmarshal(fmt, []byte(expected))
				require.NoError(err)
				if test.overrideUnmarshalledDef != nil {
					require.Equal(*test.overrideUnmarshalledDef, d)
				} else {
					require.Equal(test.def, d)
				}
			}
		})
	}
}
