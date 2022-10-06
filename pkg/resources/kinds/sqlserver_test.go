package kinds

import (
	"testing"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/stretchr/testify/require"
)

func TestSQLServerResource(t *testing.T) {
	require := require.New(t)

	resource := SQLServerResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSQLServer,
		},
		Host:        "hostname",
		Port:        "5432",
		Database:    "database",
		Username:    "username",
		Password:    "password",
		EncryptMode: "true",
	}
	err := resource.Calculate()
	require.NoError(err)
	err = resource.Validate()
	require.NoError(err)

	// Update without sensitive info, sensitive info should still be there.
	err = resource.Update(&SQLServerResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSQLServer,
		},
		Host:        "host",
		Port:        "5432",
		Database:    "db",
		Username:    "username",
		EncryptMode: "disable",
	})
	require.NoError(err)
	require.Equal("host", resource.Host)
	require.Equal("5432", resource.Port)
	require.Equal("db", resource.Database)
	require.Equal("username", resource.Username)
	require.Equal("password", resource.Password)
	require.Equal("disable", resource.EncryptMode)
	require.NotEmpty(resource.DSN)
	err = resource.Validate()
	require.NoError(err)

	// Update with sensitive info, sensitive info should be changed.
	err = resource.Update(&SQLServerResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSQLServer,
		},
		Host:        "hostname",
		Port:        "5432",
		Database:    "database",
		Username:    "user",
		Password:    "pass",
		EncryptMode: "true",
	})
	require.NoError(err)
	require.Equal("hostname", resource.Host)
	require.Equal("5432", resource.Port)
	require.Equal("database", resource.Database)
	require.Equal("user", resource.Username)
	require.Equal("pass", resource.Password)
	require.Equal("true", resource.EncryptMode)
	require.NotEmpty(resource.DSN)
	err = resource.Validate()
	require.NoError(err)
}
