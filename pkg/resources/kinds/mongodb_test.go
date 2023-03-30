package kinds

import (
	"testing"

	"github.com/airplanedev/cli/pkg/resources"
	"github.com/stretchr/testify/require"
)

func TestMongoDBResource(t *testing.T) {
	require := require.New(t)

	resource := MongoDBResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindMongoDB,
		},
		ConnectionString: "mongodb://username:password@hostname:27017/db",
	}
	err := resource.Calculate()
	require.NoError(err)
	err = resource.Validate()
	require.NoError(err)

	// Update without sensitive info, sensitive info should still be there.
	err = resource.Update(&MongoDBResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindMongoDB,
		},
	})
	require.NoError(err)
	require.Equal("mongodb://username:password@hostname:27017/db", resource.ConnectionString)
	err = resource.Validate()
	require.NoError(err)

	// Update with sensitive info, sensitive info should be changed.
	err = resource.Update(&MongoDBResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindMongoDB,
		},
		ConnectionString: "mongodb://username:password@host:27017/my_db",
	})
	require.NoError(err)
	require.Equal("mongodb://username:password@host:27017/my_db", resource.ConnectionString)
	err = resource.Validate()
	require.NoError(err)
}
