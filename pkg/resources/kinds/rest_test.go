package kinds

import (
	"testing"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/utils/pointers"
	"github.com/stretchr/testify/require"
)

func TestRESTResource(t *testing.T) {
	require := require.New(t)

	resource := RESTResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindREST,
		},
		BaseURL: "http://example.com",
		Headers: map[string]string{
			"my_header":        "my_header_value",
			"my_secret_header": "my_secret_header_value",
		},
		SecretHeaders: []string{
			"my_secret_header",
		},
	}
	err := resource.Calculate()
	require.NoError(err)
	err = resource.Validate()
	require.NoError(err)

	// Update without sensitive info, sensitive info should still be there.
	err = resource.Update(&RESTResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindREST,
		},
		BaseURL: "http://example.com",
		Headers: map[string]string{
			"my_header":        "my_header_value",
			"my_secret_header": "",
		},
		SecretHeaders: []string{
			"my_secret_header",
		},
	})
	require.NoError(err)
	require.Equal("http://example.com", resource.BaseURL)
	require.Equal("my_header_value", resource.Headers["my_header"])
	require.Equal("my_secret_header_value", resource.Headers["my_secret_header"])
	require.Equal(1, len(resource.SecretHeaders))
	require.Equal("my_secret_header", resource.SecretHeaders[0])
	require.Nil(resource.Auth)
	err = resource.Validate()
	require.NoError(err)

	// Update with sensitive info, sensitive info should be changed.
	err = resource.Update(&RESTResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindREST,
		},
		BaseURL: "http://example.com",
		Headers: map[string]string{
			"my_header":        "my_header_value",
			"my_secret_header": "my_new_secret_header_value",
		},
		SecretHeaders: []string{
			"my_secret_header",
		},
	})
	require.NoError(err)
	require.Equal("http://example.com", resource.BaseURL)
	require.Equal("my_header_value", resource.Headers["my_header"])
	require.Equal("my_new_secret_header_value", resource.Headers["my_secret_header"])
	require.Equal(1, len(resource.SecretHeaders))
	require.Equal("my_secret_header", resource.SecretHeaders[0])
	require.Nil(resource.Auth)
	err = resource.Validate()
	require.NoError(err)

	// Change the auth.
	err = resource.Update(&RESTResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindREST,
		},
		BaseURL: "http://example.com",
		Headers: map[string]string{
			"my_header":        "my_header_value",
			"my_secret_header": "my_new_secret_header_value",
		},
		SecretHeaders: []string{
			"my_secret_header",
		},
		Auth: &RESTAuthBasic{
			Kind:     RESTAuthKindBasic,
			Username: pointers.String("username"),
			Password: pointers.String("password"),
		},
	})
	require.NoError(err)
	auth, ok := resource.Auth.(*RESTAuthBasic)
	require.True(ok)
	require.NotNil(auth.Username)
	require.NotNil(auth.Password)
	require.Equal("username", *auth.Username)
	require.Equal("password", *auth.Password)
	require.Equal(1, len(auth.Headers))
	require.Equal("Basic dXNlcm5hbWU6cGFzc3dvcmQ=", auth.Headers["Authorization"])
	err = resource.Validate()
	require.NoError(err)

	// Scrub calculated fields
	resource.ScrubCalculatedFields()
	require.Empty(auth.Headers)

	// Update the resource, but not the auth.
	err = resource.Update(&RESTResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindREST,
		},
		BaseURL: "http://example.com/foo",
		Headers: map[string]string{
			"my_header":        "my_header_value",
			"my_secret_header": "my_new_secret_header_value",
		},
		SecretHeaders: []string{
			"my_secret_header",
		},
		Auth: &RESTAuthBasic{
			Kind: RESTAuthKindBasic,
		},
	})
	require.NoError(err)
	auth, ok = resource.Auth.(*RESTAuthBasic)
	require.True(ok)
	require.NotNil(auth.Username)
	require.NotNil(auth.Password)
	require.Equal("username", *auth.Username)
	require.Equal("password", *auth.Password)
	require.Equal(1, len(auth.Headers))
	require.Equal("Basic dXNlcm5hbWU6cGFzc3dvcmQ=", auth.Headers["Authorization"])
	err = resource.Validate()
	require.NoError(err)

	// Back to no auth.
	err = resource.Update(&RESTResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindREST,
		},
		BaseURL: "http://example.com/foo",
		Headers: map[string]string{
			"my_header":        "my_header_value",
			"my_secret_header": "my_new_secret_header_value",
		},
		SecretHeaders: []string{
			"my_secret_header",
		},
	})
	require.NoError(err)
	require.Nil(resource.Auth)
	err = resource.Validate()
	require.NoError(err)
}
