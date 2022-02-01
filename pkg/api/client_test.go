package api

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeQueryStringest(tt *testing.T) {
	tests := []struct {
		desc   string
		path   string
		params url.Values
		out    string
	}{
		{
			desc:   "no params",
			path:   "/test",
			params: url.Values{},
			out:    "/test",
		},
		{
			desc: "one empty param",
			path: "/test",
			params: url.Values{
				"foo": []string{""},
			},
			out: "/test",
		},
		{
			desc: "mix of params",
			path: "/test",
			params: url.Values{
				"zero": []string{},
				"one":  []string{"a"},
				"two":  []string{"a", "b"},
				// Empty strings should be included if there are more than one element to retain ordering
				"three": []string{"a", "b", ""},
			},
			// Query params are sorted alphabetically:
			out: "/test?one=a&three=a&three=b&three=&two=a&two=b",
		},
	}
	for _, test := range tests {
		tt.Run(test.desc, func(t *testing.T) {
			require.Equal(t, test.out, encodeQueryString(test.path, test.params))
		})
	}
}
