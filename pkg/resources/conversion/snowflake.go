package conversion

import (
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/airplanedev/lib/pkg/utils/logger"
)

func ConvertSnowflakeResource(r *kinds.SnowflakeResource, l logger.Logger) (kind_configs.InternalResource, error) {
	return kind_configs.InternalResource{
		ID:   r.ID(),
		Slug: r.Slug,
		Name: r.Name,
		Kind: r.Kind(),
		KindConfig: kind_configs.ResourceKindConfig{
			Snowflake: &kind_configs.SnowflakeKindConfig{
				Account:   r.Account,
				Warehouse: r.Warehouse,
				Database:  r.Database,
				Schema:    r.Schema,
				Role:      r.Role,
				Username:  r.Username,
				Password:  r.Password,
			},
		},
	}, nil
}
