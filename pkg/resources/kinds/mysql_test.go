package kinds

import (
	"testing"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/stretchr/testify/require"
)

func TestMySQLResource(t *testing.T) {
	require := require.New(t)

	resource := MySQLResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindMySQL,
		},
		Host:     "hostname",
		Port:     "5432",
		Database: "database",
		Username: "username",
		Password: "password",
		TLS:      "skip-verify",
	}
	err := resource.Calculate()
	require.NoError(err)
	err = resource.Validate()
	require.NoError(err)

	// Update without sensitive info, sensitive info should still be there.
	err = resource.Update(&MySQLResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindMySQL,
		},
		Host:     "host",
		Port:     "5432",
		Database: "db",
		Username: "username",
		TLS:      "false",
	})
	require.NoError(err)
	require.Equal("host", resource.Host)
	require.Equal("5432", resource.Port)
	require.Equal("db", resource.Database)
	require.Equal("username", resource.Username)
	require.Equal("password", resource.Password)
	require.Equal("false", resource.TLS)
	require.NotEmpty(resource.DSN)
	err = resource.Validate()
	require.NoError(err)

	// Update with sensitive info, sensitive info should be changed.
	err = resource.Update(&MySQLResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindMySQL,
		},
		Host:     "hostname",
		Port:     "5432",
		Database: "database",
		Username: "user",
		Password: "pass",
		TLS:      "skip-verify",
	})
	require.NoError(err)
	require.Equal("hostname", resource.Host)
	require.Equal("5432", resource.Port)
	require.Equal("database", resource.Database)
	require.Equal("user", resource.Username)
	require.Equal("pass", resource.Password)
	require.Equal("skip-verify", resource.TLS)
	require.NotEmpty(resource.DSN)
	err = resource.Validate()
	require.NoError(err)
}
