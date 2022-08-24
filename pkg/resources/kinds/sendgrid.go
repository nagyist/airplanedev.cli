package kinds

import (
	"github.com/airplanedev/lib/pkg/resources"
)

var ResourceKindSendGrid resources.ResourceKind = "sendgrid"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindSendGrid, func() resources.Resource { return SendGridResource{} })
}

type SendGridResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	APIKey string `json:"apiKey" mapstructure:"apiKey"`
}

var _ resources.Resource = SendGridResource{}

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
