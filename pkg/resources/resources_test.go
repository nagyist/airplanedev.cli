package resources

import (
	"context"
	"testing"

	"github.com/airplanedev/cli/pkg/dev/env"
	libresources "github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/stretchr/testify/require"
)

func TestGenerateAliasToResourceMap(t *testing.T) {
	require := require.New(t)

	demoDBResource := env.ResourceWithEnv{
		Resource: &kinds.PostgresResource{
			BaseResource: libresources.BaseResource{
				Kind: kinds.ResourceKindPostgres,
				Slug: "demo_db",
			},
			Username: "postgres",
			Password: "password",
			Port:     "5432",
			SSLMode:  "disable",
		},
		Remote: false,
	}
	dBResource := env.ResourceWithEnv{
		Resource: &kinds.PostgresResource{
			BaseResource: libresources.BaseResource{
				Kind: kinds.ResourceKindPostgres,
				Slug: "db",
			},
			Username: "admin",
			Password: "password",
			Port:     "5432",
			SSLMode:  "disable",
		},
		Remote: false,
	}

	aliasToResourceMap, err := GenerateAliasToResourceMap(
		context.Background(),
		map[string]string{"demo_db": "demo_db", "my_db": "db"},
		map[string]env.ResourceWithEnv{
			"demo_db": demoDBResource,
			"db":      dBResource,
		},
		nil,
		nil,
	)

	require.NoError(err)
	require.Equal(map[string]libresources.Resource{
		"demo_db": demoDBResource.Resource,
		"my_db":   dBResource.Resource,
	}, aliasToResourceMap)
}
