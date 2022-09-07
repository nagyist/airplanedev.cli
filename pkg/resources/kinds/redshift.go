package kinds

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindRedshift resources.ResourceKind = "redshift"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindRedshift, func() resources.Resource { return &RedshiftResource{} })
}

type RedshiftResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`
	PostgresResource       `mapstructure:",squash" yaml:",inline"`
}

var _ SQLResourceInterface = &RedshiftResource{}

func (r *RedshiftResource) ScrubSensitiveData() {
	r.PostgresResource.ScrubSensitiveData()
}

func (r *RedshiftResource) Update(other resources.Resource) error {
	o, ok := other.(*RedshiftResource)
	if !ok {
		return errors.Errorf("expected *RedshiftResource got %T", other)
	}

	return r.PostgresResource.Update(&o.PostgresResource)
}

func (r RedshiftResource) Validate() error {
	return r.PostgresResource.Validate()
}

func (r RedshiftResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r RedshiftResource) String() string {
	return r.PostgresResource.String()
}

func (r RedshiftResource) GetDSN() string {
	return r.PostgresResource.GetDSN()
}

func (r RedshiftResource) GetSSHConfig() *SSHConfig {
	return r.PostgresResource.GetSSHConfig()
}

func (r RedshiftResource) GetSQLDriver() SQLDriver {
	return r.PostgresResource.GetSQLDriver()
}

func (r RedshiftResource) ID() string {
	return r.BaseResource.ID
}
