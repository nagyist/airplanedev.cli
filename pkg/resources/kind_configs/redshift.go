package kind_configs

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

const (
	KindRedshift resources.ResourceKind = "redshift"
)

func init() {
	ResourceKindToKindConfig[KindRedshift] = &RedshiftKindConfig{}
}

type RedshiftKindConfig struct {
	PostgresKindConfig `yaml:",inline"`
}

var _ ResourceConfigValues = &RedshiftKindConfig{}

func (this *RedshiftKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*RedshiftKindConfig)
	if !ok {
		return errors.Errorf("expected *RedshiftKindConfig got %T", cv)
	}
	this.SqlBaseConfig.update(c.SqlBaseConfig)
	return nil
}

func (this RedshiftKindConfig) Validate() error {
	return this.PostgresKindConfig.Validate()
}

func (this RedshiftKindConfig) sslModeString() string { // nolint:unused
	return this.PostgresKindConfig.sslModeString()
}

func (this RedshiftKindConfig) dsn() string { // nolint:unused
	return this.PostgresKindConfig.dsn()
}

func (this RedshiftKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	return this.PostgresKindConfig.ToExternalResource(base)
}
