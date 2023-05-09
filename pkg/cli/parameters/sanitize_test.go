package parameters

import (
	"testing"

	"github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/stretchr/testify/require"
)

func TestSanitizeParamValues(t *testing.T) {
	for _, test := range []struct {
		name     string
		values   map[string]interface{}
		params   api.Parameters
		expected map[string]interface{}
	}{
		{
			name: "lower slug",
			values: map[string]interface{}{
				"password":      "foo",
				"token":         123,
				"authorization": true,
				"secret":        map[string]interface{}{"foo": "bar"},
				"api_key":       []interface{}{"foo"},
			},
			expected: map[string]interface{}{
				"password":      "****",
				"token":         0,
				"authorization": false,
				"secret":        map[string]interface{}{},
				"api_key":       []interface{}{},
			},
		},
		{
			name: "name",
			values: map[string]interface{}{
				"one":   "foo",
				"two":   123,
				"three": true,
				"four":  map[string]interface{}{"foo": "bar"},
				"five":  []interface{}{"foo"},
			},
			params: api.Parameters{
				{
					Slug: "one",
					Name: "password",
				},
				{
					Slug: "two",
					Name: "token",
				},
				{
					Slug: "three",
					Name: "authorization",
				},
				{
					Slug: "four",
					Name: "secret",
				},
				{
					Slug: "five",
					Name: "api key",
				},
			},
			expected: map[string]interface{}{
				"one":   "****",
				"two":   0,
				"three": false,
				"four":  map[string]interface{}{},
				"five":  []interface{}{},
			},
		},
		{
			name: "case insensitive name",
			values: map[string]interface{}{
				"one":   "foo",
				"two":   123,
				"three": true,
				"four":  map[string]interface{}{"foo": "bar"},
				"five":  []interface{}{"foo"},
			},
			params: api.Parameters{
				{
					Slug: "one",
					Name: "Password",
				},
				{
					Slug: "two",
					Name: "Token",
				},
				{
					Slug: "three",
					Name: "Authorization",
				},
				{
					Slug: "four",
					Name: "Secret",
				},
				{
					Slug: "five",
					Name: "API key",
				},
			},
			expected: map[string]interface{}{
				"one":   "****",
				"two":   0,
				"three": false,
				"four":  map[string]interface{}{},
				"five":  []interface{}{},
			},
		},
		{
			name: "no sanitization",
			values: map[string]interface{}{
				"one":   "foo",
				"two":   123,
				"three": true,
				"four":  map[string]interface{}{"foo": "bar"},
				"five":  []interface{}{"foo"},
			},
			params: api.Parameters{
				{
					Slug: "one",
					Name: "One",
				},
				{
					Slug: "two",
					Name: "Two",
				},
				{
					Slug: "three",
					Name: "Three",
				},
				{
					Slug: "four",
					Name: "Four",
				},
				{
					Slug: "five",
					Name: "Five",
				},
			},
			expected: map[string]interface{}{
				"one":   "foo",
				"two":   123,
				"three": true,
				"four":  map[string]interface{}{"foo": "bar"},
				"five":  []interface{}{"foo"},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)
			result, err := SanitizeParamValues(test.values, test.params)
			require.NoError(err)
			require.Equal(test.expected, result)
		})
	}
}
