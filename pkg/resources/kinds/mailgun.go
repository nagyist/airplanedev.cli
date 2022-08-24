package kinds

import (
	"fmt"

	"github.com/airplanedev/lib/pkg/resources"
)

var ResourceKindMailgun resources.ResourceKind = "mailgun"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindMailgun, func() resources.Resource { return MailgunResource{} })
}

type MailgunResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	APIKey string `json:"apiKey" mapstructure:"apiKey"`
	Domain string `json:"domain" mapstructure:"domain"`
}

var _ resources.Resource = MailgunResource{}

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
