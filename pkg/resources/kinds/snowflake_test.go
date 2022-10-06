package kinds

import (
	"testing"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/stretchr/testify/require"
)

func TestSnowflakeResource(t *testing.T) {
	require := require.New(t)

	resource := SnowflakeResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSnowflake,
		},
		Account:   "my_account",
		Warehouse: "my_warehouse",
		Database:  "my_database",
		Schema:    "my_schema",
		Role:      "my_role",
		Username:  "username",
		Password:  "password",
	}
	err := resource.Calculate()
	require.NoError(err)
	err = resource.Validate()
	require.NoError(err)

	// Update without sensitive info, sensitive info should still be there.
	err = resource.Update(&SnowflakeResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSnowflake,
		},
		Account:   "my_account",
		Warehouse: "my_warehouse",
		Database:  "my_database",
		Schema:    "my_schema",
		Role:      "my_role",
		Username:  "username",
	})
	require.NoError(err)
	require.Equal("my_account", resource.Account)
	require.Equal("my_warehouse", resource.Warehouse)
	require.Equal("my_database", resource.Database)
	require.Equal("my_schema", resource.Schema)
	require.Equal("my_role", resource.Role)
	require.Equal("username", resource.Username)
	require.Equal("password", resource.Password)
	require.NotEmpty(resource.DSN)
	err = resource.Validate()
	require.NoError(err)

	// Update with sensitive info, sensitive info should be changed.
	err = resource.Update(&SnowflakeResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindSnowflake,
		},
		Account:   "my_account",
		Warehouse: "my_warehouse",
		Database:  "my_database",
		Schema:    "my_schema",
		Role:      "my_role",
		Username:  "user",
		Password:  "pass",
	})
	require.NoError(err)
	require.Equal("my_account", resource.Account)
	require.Equal("my_warehouse", resource.Warehouse)
	require.Equal("my_database", resource.Database)
	require.Equal("my_schema", resource.Schema)
	require.Equal("my_role", resource.Role)
	require.Equal("user", resource.Username)
	require.Equal("pass", resource.Password)
	require.NotEmpty(resource.DSN)
	err = resource.Validate()
	require.NoError(err)
}
