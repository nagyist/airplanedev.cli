package conversion

import (
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

func ConvertSMTPResource(r kinds.SMTPResource, l logger.Logger) (kind_configs.InternalResource, error) {
	authConfig := kind_configs.SMTPAuthConfig{}
	switch auth := r.Auth.(type) {
	case kinds.SMTPAuthPlain:
		authConfig.Plain = &kind_configs.SMTPAuthConfigPlain{
			Identity: auth.Identity,
			Username: auth.Username,
			Password: auth.Password,
		}
	case kinds.SMTPAuthCRAMMD5:
		authConfig.CRAMMD5 = &kind_configs.SMTPAuthConfigCRAMMD5{
			Username: auth.Username,
			Secret:   auth.Secret,
		}
	case kinds.SMTPAuthLogin:
		authConfig.Login = &kind_configs.SMTPAuthConfigLogin{
			Username: auth.Username,
			Password: auth.Password,
		}
	default:
		return kind_configs.InternalResource{}, errors.Errorf("Unknown SMTP auth kind: %T", r.Auth)
	}

	return kind_configs.InternalResource{
		ID:   r.ID(),
		Slug: r.Slug,
		Name: r.Name,
		Kind: r.Kind(),
		KindConfig: kind_configs.ResourceKindConfig{
			SMTP: &kind_configs.SMTPKindConfig{
				AuthConfig: authConfig,
				Hostname:   r.Hostname,
				Port:       r.Port,
			},
		},
	}, nil
}
