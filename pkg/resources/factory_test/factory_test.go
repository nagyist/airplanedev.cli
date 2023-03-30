// This is pulled out into a separate package to avoid cyclic dependency problems.
package factory_test

import (
	"encoding/json"
	"testing"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/airplanedev/lib/pkg/utils/pointers"
	"github.com/stretchr/testify/require"
)

func TestResourceFactories(t *testing.T) {
	for _, test := range []struct {
		name     string
		resource resources.Resource
	}{
		{
			name: "bigquery",
			resource: &kinds.BigQueryResource{
				BaseResource: resources.BaseResource{
					Kind: "bigquery",
					ID:   "bigquery id",
					Slug: "bigquery",
					Name: "BigQuery",
				},
				Credentials:    "credentials",
				RawCredentials: "raw-credentials",
				ProjectID:      "project-id",
				DataSet:        "dataset",
				Location:       "location",
				DSN:            "dsn",
			},
		},
		{
			name: "graphql",
			resource: &kinds.GraphQLResource{
				BaseResource: resources.BaseResource{
					Kind: "graphql",
					ID:   "graphql id",
					Slug: "graphql",
					Name: "GraphQL",
				},
				RESTResource: kinds.RESTResource{
					BaseResource: resources.BaseResource{
						Kind: "graphql",
						ID:   "graphql id",
						Slug: "graphql",
						Name: "GraphQL",
					},
					BaseURL: "http://example.com",
					Headers: map[string]string{
						"header": "value",
					},
					SecretHeaders: []string{"secret"},
					Auth: &kinds.RESTAuthBasic{
						Kind:     kinds.RESTAuthKindBasic,
						Username: pointers.String("username"),
						Password: pointers.String("password"),
					},
				},
			},
		},
		{
			name: "rest",
			resource: &kinds.RESTResource{
				BaseResource: resources.BaseResource{
					Kind: "rest",
					ID:   "rest id",
					Slug: "rest",
					Name: "REST",
				},
				BaseURL: "http://example.com",
				Headers: map[string]string{
					"header": "value",
				},
				SecretHeaders: []string{"secret"},
				Auth: &kinds.RESTAuthBasic{
					Kind:     kinds.RESTAuthKindBasic,
					Username: pointers.String("username"),
					Password: pointers.String("password"),
				},
			},
		},
		{
			name: "mailgun",
			resource: &kinds.MailgunResource{
				BaseResource: resources.BaseResource{
					Kind: "mailgun",
					ID:   "mailgun id",
					Slug: "mailgun_email",
					Name: "Mailgun Email",
				},
				APIKey: "api-key",
				Domain: "domain",
			},
		},
		{
			name: "mongodb",
			resource: &kinds.MongoDBResource{
				BaseResource: resources.BaseResource{
					Kind: "mongodb",
					ID:   "mongodb id",
					Slug: "mongodb",
					Name: "MongoDB",
				},
				ConnectionString: "connection-string",
			},
		},
		{
			name: "mysql",
			resource: &kinds.MySQLResource{
				BaseResource: resources.BaseResource{
					Kind: "mysql",
					ID:   "mysql_id",
					Slug: "mysql",
					Name: "MySQL",
				},
				Username:      "username",
				Password:      "password",
				Host:          "host",
				Port:          "port",
				Database:      "database",
				TLS:           "enabled",
				DSN:           "dsn",
				SSHHost:       "host",
				SSHPort:       "22",
				SSHUsername:   "ssh-username",
				SSHPrivateKey: "ssh-private-key",
			},
		},
		{
			name: "postgres",
			resource: &kinds.PostgresResource{
				BaseResource: resources.BaseResource{
					Kind: "postgres",
					ID:   "postgres id",
					Slug: "postgres",
					Name: "Postgres",
				},
				Username:      "username",
				Password:      "password",
				Host:          "host",
				Port:          "port",
				Database:      "database",
				SSLMode:       "sslmode",
				DSN:           "dsn",
				SSHHost:       "host",
				SSHPort:       "22",
				SSHUsername:   "ssh-username",
				SSHPrivateKey: "ssh-private-key",
			},
		},
		{
			name: "redshift",
			resource: &kinds.RedshiftResource{
				BaseResource: resources.BaseResource{
					Kind: "redshift",
					ID:   "redshift id",
					Slug: "redshift",
					Name: "redshift",
				},
				PostgresResource: kinds.PostgresResource{
					BaseResource: resources.BaseResource{
						Kind: "redshift",
						ID:   "redshift id",
						Slug: "redshift",
						Name: "redshift",
					},
					Username:      "username",
					Password:      "password",
					Host:          "host",
					Port:          "port",
					Database:      "database",
					SSLMode:       "sslmode",
					DSN:           "dsn",
					SSHHost:       "host",
					SSHPort:       "22",
					SSHUsername:   "ssh-username",
					SSHPrivateKey: "ssh-private-key",
				},
			},
		},
		{
			name: "sendgrid",
			resource: &kinds.SendGridResource{
				BaseResource: resources.BaseResource{
					Kind: "sendgrid",
					ID:   "sendgrid id",
					Slug: "sendgrid_email",
					Name: "Sendgrid Email",
				},
				APIKey: "api-key",
			},
		},
		{
			name: "slack",
			resource: &kinds.SlackResource{
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
			resource: &kinds.SMTPResource{
				BaseResource: resources.BaseResource{
					Kind: "smtp",
					ID:   "smtp id",
					Slug: "smtp_email",
					Name: "SMTP Email",
				},
				Hostname: "hostname",
				Port:     "port",
				Auth: &kinds.SMTPAuthPlain{
					Kind:     "plain",
					Username: "username",
					Password: "password",
				},
			},
		},
		{
			name: "snowflake",
			resource: &kinds.SnowflakeResource{
				BaseResource: resources.BaseResource{
					Kind: "snowflake",
					ID:   "snowflake id",
					Slug: "snowflake",
					Name: "Snowflake",
				},
				Account:   "account",
				Warehouse: "warehouse",
				Database:  "database",
				Schema:    "schema",
				Role:      "role",
				Username:  "username",
				Password:  "password",
				DSN:       "dsn",
			},
		},
		{
			name: "sqlserver",
			resource: &kinds.SQLServerResource{
				BaseResource: resources.BaseResource{
					Kind: "sqlserver",
					ID:   "sqlserver_id",
					Slug: "sqlserver",
					Name: "sqlserver",
				},
				Username:      "username",
				Password:      "password",
				Host:          "host",
				Port:          "port",
				Database:      "database",
				EncryptMode:   "true",
				DSN:           "dsn",
				SSHHost:       "host",
				SSHPort:       "22",
				SSHUsername:   "ssh-username",
				SSHPrivateKey: "ssh-private-key",
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
