// This is pulled out into a separate package to avoid cyclic dependency problems.
package factory_test

import (
	"encoding/json"
	"testing"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/stretchr/testify/require"
)

func TestResourceFactories(t *testing.T) {
	for _, test := range []struct {
		name     string
		resource resources.Resource
	}{
		{
			name: "slack",
			resource: kinds.SlackResource{
				BaseResource: resources.BaseResource{
					Kind: "slack",
					ID:   "slack id",
					Slug: "slack",
					Name: "Slack",
				},
				AccessToken: "access_token",
			},
		},
		{
			name: "smtp",
			resource: kinds.SMTPResource{
				BaseResource: resources.BaseResource{
					Kind: "smtp",
					ID:   "smtp id",
					Slug: "smtp_email",
					Name: "SMTP Email",
				},
				Hostname: "hostname",
				Port:     "port",
				Auth: kinds.SMTPAuthPlain{
					Kind:     "plain",
					Username: "username",
					Password: "password",
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			b, err := json.Marshal(test.resource)
			require.NoError(err)

			m := map[string]interface{}{}
			err = json.Unmarshal(b, &m)
			require.NoError(err)

			kind, ok := m["kind"]
			require.True(ok)
			kindStr, ok := kind.(string)
			require.True(ok)

			rehydrated, err := resources.GetResource(resources.ResourceKind(kindStr), m)
			require.NoError(err)
			require.Equal(test.resource, rehydrated)
		})
	}
}
