package builtins

import (
	"testing"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/stretchr/testify/require"
)

func TestGetFunctionSpecificationFromKindOptions(t *testing.T) {
	for _, test := range []struct {
		name                  string
		kindOptions           build.KindOptions
		functionSpecification *FunctionSpecification
	}{
		{
			name: "well-formed",
			kindOptions: build.KindOptions{
				"functionSpecification": map[string]interface{}{
					"namespace": "sql",
					"name":      "query",
				},
			},
			functionSpecification: &FunctionSpecification{
				Namespace: "sql",
				Name:      "query",
			},
		},
		{
			name:        "missing",
			kindOptions: build.KindOptions{},
		},
		{
			name: "missing name",
			kindOptions: build.KindOptions{
				"functionSpecification": map[string]interface{}{
					"namespace": "sql",
				},
			},
		},
		{
			name: "missing namespace",
			kindOptions: build.KindOptions{
				"functionSpecification": map[string]interface{}{
					"name": "query",
				},
			},
		},
		{
			name: "wrong type",
			kindOptions: build.KindOptions{
				"functionSpecification": "sql.query",
			},
		},
		{
			name: "name wrong type",
			kindOptions: build.KindOptions{
				"functionSpecification": map[string]interface{}{
					"namespace": "sql",
					"name":      8,
				},
			},
		},
		{
			name: "namespace wrong type",
			kindOptions: build.KindOptions{
				"functionSpecification": map[string]interface{}{
					"namespace": 100,
					"name":      "query",
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)
			out, err := GetFunctionSpecificationFromKindOptions(test.kindOptions)
			if test.functionSpecification != nil {
				require.NoError(err)
				require.Equal(out, *test.functionSpecification)
			} else {
				require.Error(err)
			}
		})
	}
}
