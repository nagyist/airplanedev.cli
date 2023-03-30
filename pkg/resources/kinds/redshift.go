package kinds

import (
	"github.com/airplanedev/cli/pkg/resources"
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

	if err := r.PostgresResource.Update(&o.PostgresResource); err != nil {
		return errors.Wrap(err, "error updating postgres resource")
	}

	if err := r.Calculate(); err != nil {
		return errors.Wrap(err, "error computing calculated fields")
	}

	return nil
}

func (r *RedshiftResource) Calculate() error {
	if err := r.PostgresResource.Calculate(); err != nil {
		return errors.Wrap(err, "error computing calculated fields on postgres resource")
	}
	return nil
}

func (r *RedshiftResource) ScrubCalculatedFields() {
	r.PostgresResource.ScrubCalculatedFields()
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

func (r *RedshiftResource) UpdateBaseResource(br resources.BaseResource) error {
	r.BaseResource.Update(br)
	return nil
}
