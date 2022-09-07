package conversion

import (
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
)

func ConvertBigQueryResource(r *kinds.BigQueryResource) (kind_configs.InternalResource, error) {
	return kind_configs.InternalResource{
		ID:   r.ID(),
		Slug: r.Slug,
		Name: r.Name,
		Kind: r.Kind(),
		KindConfig: kind_configs.ResourceKindConfig{
			BigQuery: &kind_configs.BigQueryKindConfig{
				Credentials: r.Credentials,
				ProjectID:   r.ProjectID,
				Location:    r.Location,
				DataSet:     r.DataSet,
			},
		},
	}, nil
}
