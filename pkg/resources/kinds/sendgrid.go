package kinds

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindSendGrid resources.ResourceKind = "sendgrid"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindSendGrid, func() resources.Resource { return &SendGridResource{} })
}

type SendGridResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	APIKey string `json:"apiKey" mapstructure:"apiKey"`
}

var _ resources.Resource = &SendGridResource{}

func (r *SendGridResource) ScrubSensitiveData() {
	r.APIKey = ""
}

func (r *SendGridResource) Update(other resources.Resource) error {
	o, ok := other.(*SendGridResource)
	if !ok {
		return errors.Errorf("expected *SendGridResource got %T", other)
	}

	if o.APIKey != "" {
		r.APIKey = o.APIKey
	}

	if err := r.Calculate(); err != nil {
		return errors.Wrap(err, "error computing calculated fields")
	}

	return nil
}

func (r *SendGridResource) Calculate() error {
	return nil
}

func (r *SendGridResource) ScrubCalculatedFields() {}

func (r SendGridResource) Validate() error {
	if r.APIKey == "" {
		return resources.NewErrMissingResourceField("apiKey")
	}

	return nil
}

func (r SendGridResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r SendGridResource) String() string {
	return "SendGridResource"
}

func (r SendGridResource) ID() string {
	return r.BaseResource.ID
}

func (r *SendGridResource) UpdateBaseResource(br resources.BaseResource) error {
	r.BaseResource.Update(br)
	return nil
}
