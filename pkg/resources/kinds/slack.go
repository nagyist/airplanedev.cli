package kinds

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindSlack resources.ResourceKind = "slack"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindSlack, func() resources.Resource { return &SlackResource{} })
}

type SlackResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	AccessToken string `json:"accessToken" mapstructure:"accessToken"`
}

var _ resources.Resource = &SlackResource{}

func (r *SlackResource) ScrubSensitiveData() {
	r.AccessToken = ""
}

func (r *SlackResource) Update(other resources.Resource) error {
	return errors.New("NotImplemented: Slack resource cannot be updated")
}

func (r *SlackResource) Calculate() error {
	return nil
}

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

func (r SlackResource) ID() string {
	return r.BaseResource.ID
}

func (r *SlackResource) UpdateBaseResource(br resources.BaseResource) error {
	return errors.New("NotImplemented: Slack resource cannot be updated")
}
