package conversion

import (
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/airplanedev/lib/pkg/utils/logger"
)

func ConvertPostgresResource(r *kinds.PostgresResource, l logger.Logger) (kind_configs.InternalResource, error) {
	return kind_configs.InternalResource{
		ID:   r.ID(),
		Slug: r.Slug,
		Name: r.Name,
		Kind: r.Kind(),
		KindConfig: kind_configs.ResourceKindConfig{
			Postgres: &kind_configs.PostgresKindConfig{
				SqlBaseConfig: kind_configs.SqlBaseConfig{
					Host:          r.Host,
					Port:          r.Port,
					Database:      r.Database,
					Username:      r.Username,
					Password:      r.Password,
					DisableSSL:    r.SSLMode == "disable",
					SSHHost:       r.SSHHost,
					SSHPort:       r.SSHPort,
					SSHUsername:   r.SSHUsername,
					SSHPrivateKey: r.SSHPrivateKey,
				},
			},
		},
	}, nil
}
