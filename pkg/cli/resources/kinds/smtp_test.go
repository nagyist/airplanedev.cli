package kinds

import (
	"testing"

	"github.com/airplanedev/cli/pkg/cli/resources"
	"github.com/stretchr/testify/require"
)

func TestSMTPResource(t *testing.T) {
	require := require.New(t)

	resource := SMTPResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSMTP,
		},
		Hostname: "hostname",
		Port:     "25",
		Auth: &SMTPAuthLogin{
			Kind:     EmailSMTPAuthKindLogin,
			Username: "username",
			Password: "password",
		},
	}
	err := resource.Calculate()
	require.NoError(err)
	err = resource.Validate()
	require.NoError(err)

	// Update without sensitive info, sensitive info should still be there.
	err = resource.Update(&SMTPResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSMTP,
		},
		Hostname: "hostname",
		Port:     "25",
		Auth: &SMTPAuthLogin{
			Kind:     EmailSMTPAuthKindLogin,
			Username: "username",
		},
	})
	require.NoError(err)
	require.Equal("hostname", resource.Hostname)
	require.Equal("25", resource.Port)
	auth, ok := resource.Auth.(*SMTPAuthLogin)
	require.True(ok)
	require.Equal("username", auth.Username)
	require.Equal("password", auth.Password)
	err = resource.Validate()
	require.NoError(err)

	// Update with sensitive info, sensitive info should be changed.
	err = resource.Update(&SMTPResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSMTP,
		},
		Hostname: "hostname",
		Port:     "25",
		Auth: &SMTPAuthLogin{
			Kind:     EmailSMTPAuthKindLogin,
			Username: "user",
			Password: "pass",
		},
	})
	require.NoError(err)
	require.Equal("hostname", resource.Hostname)
	require.Equal("25", resource.Port)
	auth, ok = resource.Auth.(*SMTPAuthLogin)
	require.True(ok)
	require.Equal("user", auth.Username)
	require.Equal("pass", auth.Password)
	err = resource.Validate()
	require.NoError(err)

	// Update with a different auth type.
	err = resource.Update(&SMTPResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSMTP,
		},
		Hostname: "hostname",
		Port:     "25",
		Auth: &SMTPAuthPlain{
			Kind:     EmailSMTPAuthKindPlain,
			Username: "my_username",
			Password: "my_password",
		},
	})
	require.NoError(err)
	require.Equal("hostname", resource.Hostname)
	require.Equal("25", resource.Port)
	plainAuth, ok := resource.Auth.(*SMTPAuthPlain)
	require.True(ok)
	require.Equal("my_username", plainAuth.Username)
	require.Equal("my_password", plainAuth.Password)
	err = resource.Validate()
	require.NoError(err)
}
