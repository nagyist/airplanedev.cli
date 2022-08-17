package kind_configs

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

const KindSlack ResourceKind = "slack"

func init() {
	ResourceKindToKindConfig[KindSlack] = &SlackKindConfig{}
}

type SlackKindConfig struct {
	AccessToken string `json:"accessToken" yaml:"accessToken"`
}

var _ ResourceConfigValues = &SlackKindConfig{}

func (kc *SlackKindConfig) Update(cv ResourceConfigValues) error {
	return errors.New("NotImplemented: Slack resource cannot be updated")
}

func (kc SlackKindConfig) Validate() error {
	r, err := kc.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func (kc SlackKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	return kinds.SlackResource{
		BaseResource: base,
		AccessToken:  kc.AccessToken,
	}, nil
}
