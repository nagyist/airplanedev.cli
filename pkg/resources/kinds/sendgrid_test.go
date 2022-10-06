package kinds

import (
	"testing"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/stretchr/testify/require"
)

func TestSendGridResource(t *testing.T) {
	require := require.New(t)

	resource := SendGridResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSendGrid,
		},
		APIKey: "my_api_key",
	}
	err := resource.Calculate()
	require.NoError(err)
	err = resource.Validate()
	require.NoError(err)

	// Update without sensitive info, sensitive info should still be there.
	err = resource.Update(&SendGridResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSendGrid,
		},
	})
	require.NoError(err)
	require.Equal("my_api_key", resource.APIKey)
	err = resource.Validate()
	require.NoError(err)

	// Update with sensitive info, sensitive info should be changed.
	err = resource.Update(&SendGridResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSendGrid,
		},
		APIKey: "my_new_api_key",
	})
	require.NoError(err)
	require.Equal("my_new_api_key", resource.APIKey)
	err = resource.Validate()
	require.NoError(err)
}
