package conversion

import (
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/airplanedev/lib/pkg/utils/logger"
)

func ConvertMailgunResource(r kinds.MailgunResource, l logger.Logger) (kind_configs.InternalResource, error) {
	return kind_configs.InternalResource{
		ID:   r.ID(),
		Slug: r.Slug,
		Name: r.Name,
		Kind: r.Kind(),
		KindConfig: kind_configs.ResourceKindConfig{
			Mailgun: &kind_configs.MailgunKindConfig{
				APIKey: r.APIKey,
				Domain: r.Domain,
			},
		},
	}, nil
}
