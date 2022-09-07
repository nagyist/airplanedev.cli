package conversion

import (
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

func ConvertRESTResource(r *kinds.RESTResource, logger logger.Logger) (kind_configs.InternalResource, error) {
	var authConfig *kind_configs.RESTAuthConfig
	switch auth := r.Auth.(type) {
	case *kinds.RESTAuthBasic:
		authConfig = &kind_configs.RESTAuthConfig{
			Kind:     kind_configs.KindBasic,
			Username: auth.Username,
			Password: auth.Password,
		}
	case nil:
		// nothing
	default:
		return kind_configs.InternalResource{}, errors.Errorf("Unknown REST auth kind: %T", r.Auth)
	}

	return kind_configs.InternalResource{
		ID:   r.ID(),
		Slug: r.Slug,
		Name: r.Name,
		Kind: r.Kind(),
		KindConfig: kind_configs.ResourceKindConfig{
			REST: &kind_configs.RESTKindConfig{
				BaseURL:       r.BaseURL,
				Headers:       r.Headers,
				SecretHeaders: r.SecretHeaders,
				AuthConfig:    authConfig,
			},
		},
	}, nil
}
