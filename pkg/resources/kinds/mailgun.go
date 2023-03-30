package kinds

import (
	"fmt"

	"github.com/airplanedev/cli/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindMailgun resources.ResourceKind = "mailgun"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindMailgun, func() resources.Resource { return &MailgunResource{} })
}

type MailgunResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	APIKey string `json:"apiKey" mapstructure:"apiKey"`
	Domain string `json:"domain" mapstructure:"domain"`
}

var _ resources.Resource = &MailgunResource{}

func (r *MailgunResource) ScrubSensitiveData() {
	r.APIKey = ""
}

func (r *MailgunResource) Update(other resources.Resource) error {
	o, ok := other.(*MailgunResource)
	if !ok {
		return errors.Errorf("expected *MailgunResource got %T", other)
	}

	if o.APIKey != "" {
		r.APIKey = o.APIKey
	}
	r.Domain = o.Domain

	if err := r.Calculate(); err != nil {
		return errors.Wrap(err, "error computing calculated fields")
	}

	return nil
}

func (r *MailgunResource) Calculate() error {
	return nil
}

func (r *MailgunResource) ScrubCalculatedFields() {}

func (r MailgunResource) Validate() error {
	if r.APIKey == "" {
		return resources.NewErrMissingResourceField("apiKey")
	}
	if r.Domain == "" {
		return resources.NewErrMissingResourceField("domain")
	}

	return nil
}

func (r MailgunResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r MailgunResource) String() string {
	return fmt.Sprintf("MailgunResource<%s>", r.Domain)
}

func (r MailgunResource) ID() string {
	return r.BaseResource.ID
}

func (r *MailgunResource) UpdateBaseResource(br resources.BaseResource) error {
	r.BaseResource.Update(br)
	return nil
}
