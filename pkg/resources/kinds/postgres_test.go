package kinds

import (
	"testing"

	"github.com/airplanedev/cli/pkg/resources"
	"github.com/stretchr/testify/require"
)

func TestPostgresResource(t *testing.T) {
	require := require.New(t)

	resource := PostgresResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindPostgres,
		},
		Host:     "hostname",
		Port:     "5432",
		Database: "database",
		Username: "username",
		Password: "password",
		SSLMode:  "require",
	}
	err := resource.Calculate()
	require.NoError(err)
	require.Equal("postgres://username:password@hostname:5432/database?application_name=Airplane&sslmode=require", resource.DSN)
	err = resource.Validate()
	require.NoError(err)

	// Update without sensitive info, sensitive info should still be there.
	err = resource.Update(&PostgresResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindPostgres,
		},
		Host:     "host",
		Port:     "5432",
		Database: "db",
		Username: "username",
		SSLMode:  "disable",
	})
	require.NoError(err)
	require.Equal("host", resource.Host)
	require.Equal("5432", resource.Port)
	require.Equal("db", resource.Database)
	require.Equal("username", resource.Username)
	require.Equal("password", resource.Password)
	require.Equal("disable", resource.SSLMode)
	require.NotEmpty(resource.DSN)
	err = resource.Validate()
	require.NoError(err)

	// Update with sensitive info, sensitive info should be changed.
	err = resource.Update(&PostgresResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindPostgres,
		},
		Host:     "hostname",
		Port:     "5432",
		Database: "database",
		Username: "user",
		Password: "pass",
		SSLMode:  "require",
	})
	require.NoError(err)
	require.Equal("hostname", resource.Host)
	require.Equal("5432", resource.Port)
	require.Equal("database", resource.Database)
	require.Equal("user", resource.Username)
	require.Equal("pass", resource.Password)
	require.Equal("require", resource.SSLMode)
	require.NotEmpty(resource.DSN)
	err = resource.Validate()
	require.NoError(err)

	// Scrub calculated fields.
	resource.ScrubCalculatedFields()
	require.Empty(resource.DSN)
}
