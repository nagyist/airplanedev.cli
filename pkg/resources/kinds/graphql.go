package kinds

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindGraphQL resources.ResourceKind = "graphql"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindGraphQL, func() resources.Resource { return &GraphQLResource{} })
}

type GraphQLResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`
	RESTResource           `mapstructure:",squash" yaml:",inline"`
}

var _ resources.Resource = &GraphQLResource{}

func (r *GraphQLResource) ScrubSensitiveData() {
	r.RESTResource.ScrubSensitiveData()
}

func (r *GraphQLResource) Update(other resources.Resource) error {
	o, ok := other.(*GraphQLResource)
	if !ok {
		return errors.Errorf("expected *GraphQLResource got %T", other)
	}

	if err := r.RESTResource.Update(&o.RESTResource); err != nil {
		return errors.Wrap(err, "error updating REST resource")
	}

	if err := r.Calculate(); err != nil {
		return errors.Wrap(err, "error computing calculated fields")
	}

	return nil
}

func (r *GraphQLResource) Calculate() error {
	if err := r.RESTResource.Calculate(); err != nil {
		return errors.Wrap(err, "error computing calculated fields on REST resource")
	}
	return nil
}

func (r GraphQLResource) Validate() error {
	return r.RESTResource.Validate()
}

func (r GraphQLResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r GraphQLResource) String() string {
	return r.RESTResource.String()
}

func (r GraphQLResource) ID() string {
	return r.BaseResource.ID
}

func (r *GraphQLResource) UpdateBaseResource(br resources.BaseResource) error {
	r.BaseResource.Update(br)
	return nil
}
