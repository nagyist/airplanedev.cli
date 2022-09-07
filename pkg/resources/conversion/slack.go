package conversion

import (
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
)

func ConvertSlackResource(r *kinds.SlackResource) (kind_configs.InternalResource, error) {
	return kind_configs.InternalResource{
		ID:   r.ID(),
		Slug: r.Slug,
		Name: r.Name,
		Kind: r.Kind(),
		KindConfig: kind_configs.ResourceKindConfig{
			Slack: &kind_configs.SlackKindConfig{AccessToken: r.AccessToken},
		},
	}, nil
}
