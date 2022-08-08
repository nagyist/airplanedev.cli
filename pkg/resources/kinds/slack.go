package kinds

import (
	"github.com/airplanedev/lib/pkg/resources"
)

var ResourceKindSlack resources.ResourceKind = "slack"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindSlack, func() resources.Resource { return SlackResource{} })
}

type SlackResource struct {
	resources.BaseResource `mapstructure:",squash"`

	AccessToken string `json:"accessToken" mapstructure:"accessToken"`
}

var _ resources.Resource = SlackResource{}

func (r SlackResource) Validate() error {
	if r.AccessToken == "" {
		return resources.NewErrMissingResourceField("accessToken")
	}

	return nil
}

func (r SlackResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r SlackResource) String() string {
	return "SlackResource"
}
