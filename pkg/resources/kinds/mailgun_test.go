package kinds

import (
	"testing"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/stretchr/testify/require"
)

func TestMailgunResource(t *testing.T) {
	require := require.New(t)

	resource := MailgunResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindMailgun,
		},
		APIKey: "my_api_key",
		Domain: "my_domain.com",
	}
	err := resource.Calculate()
	require.NoError(err)
	err = resource.Validate()
	require.NoError(err)

	// Update without sensitive info, sensitive info should still be there.
	err = resource.Update(&MailgunResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindMailgun,
		},
		Domain: "my_domain.com",
	})
	require.NoError(err)
	require.Equal("my_api_key", resource.APIKey)
	require.Equal("my_domain.com", resource.Domain)
	err = resource.Validate()
	require.NoError(err)

	// Update with sensitive info, sensitive info should be changed.
	err = resource.Update(&MailgunResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindMailgun,
		},
		APIKey: "my_new_api_key",
		Domain: "my_new_domain.com",
	})
	require.NoError(err)
	require.Equal("my_new_api_key", resource.APIKey)
	require.Equal("my_new_domain.com", resource.Domain)
	err = resource.Validate()
	require.NoError(err)
}
