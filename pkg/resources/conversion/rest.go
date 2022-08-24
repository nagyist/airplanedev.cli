package conversion

import (
	"encoding/base64"
	"strings"

	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

func ConvertRESTResource(r kinds.RESTResource, logger logger.Logger) (kind_configs.InternalResource, error) {
	var authConfig *kind_configs.RESTAuthConfig
	if auth, ok := r.Headers["Authorization"]; ok {
		authParts := strings.Split(auth, " ")
		if len(authParts) != 2 {
			logger.Warning("authorization header is not of the form `<kind> <token>`")
		} else {
			switch authKind := authParts[0]; authKind {
			case "Basic":
				token := authParts[1]
				credentials, err := base64.StdEncoding.DecodeString(token)
				if err != nil {
					logger.Warning("error decoding authorization token: %v", err)
					break
				}

				credentialParts := strings.Split(string(credentials), ":")
				if len(credentialParts) != 2 {
					logger.Warning("credentials is not of the form <username>:<password>")
					break
				}

				authConfig = &kind_configs.RESTAuthConfig{
					Kind:     kind_configs.KindBasic,
					Username: &credentialParts[0],
					Password: &credentialParts[1],
				}
			default:
				return kind_configs.InternalResource{}, errors.Errorf("unknown auth kind: %s", authKind)
			}
		}
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
