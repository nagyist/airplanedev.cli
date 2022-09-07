package conversion

import (
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

func ConvertRESTResource(r *kinds.RESTResource) (kind_configs.InternalResource, error) {
	headers := map[string]string{}
	for k, v := range r.Headers {
		headers[k] = v
	}
	var authConfig *kind_configs.RESTAuthConfig
	switch auth := r.Auth.(type) {
	case *kinds.RESTAuthBasic:
		authConfig = &kind_configs.RESTAuthConfig{
			Kind:     kind_configs.KindBasic,
			Username: auth.Username,
			Password: auth.Password,
		}
		for authKey, authValue := range auth.Headers {
			if v, ok := headers[authKey]; ok && v == authValue {
				delete(headers, authKey)
			}
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
				Headers:       headers,
				SecretHeaders: r.SecretHeaders,
				AuthConfig:    authConfig,
			},
		},
	}, nil
}
