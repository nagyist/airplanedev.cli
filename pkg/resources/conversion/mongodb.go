package conversion

import (
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
)

func ConvertMongoDBResource(r *kinds.MongoDBResource) (kind_configs.InternalResource, error) {
	return kind_configs.InternalResource{
		ID:   r.ID(),
		Slug: r.Slug,
		Name: r.Name,
		Kind: r.Kind(),
		KindConfig: kind_configs.ResourceKindConfig{
			MongoDB: &kind_configs.MongoDBKindConfig{ConnectionString: r.ConnectionString},
		},
	}, nil
}
