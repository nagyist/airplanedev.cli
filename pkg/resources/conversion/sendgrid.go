package conversion

import (
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
)

func ConvertSendGridResource(r *kinds.SendGridResource) (kind_configs.InternalResource, error) {
	return kind_configs.InternalResource{
		ID:   r.ID(),
		Slug: r.Slug,
		Name: r.Name,
		Kind: r.Kind(),
		KindConfig: kind_configs.ResourceKindConfig{
			SendGrid: &kind_configs.SendGridKindConfig{APIKey: r.APIKey},
		},
	}, nil
}
