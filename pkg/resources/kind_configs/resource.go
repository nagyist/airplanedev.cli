package kind_configs

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

// ResourceKindToKindConfig is a mapping from ResourceKind to an empty kind config struct for that resource kind.
var ResourceKindToKindConfig = make(map[resources.ResourceKind]ResourceConfigValues)

type InternalResource struct {
	ID             string                 `json:"id" db:"id"`
	Slug           string                 `json:"slug" db:"slug"`
	Name           string                 `json:"name" db:"name"`
	Kind           resources.ResourceKind `json:"kind" db:"kind"`
	KindConfig     ResourceKindConfig     `json:"kindConfig" db:"kind_config"`
	ExportResource resources.Resource     `json:"resource"`
}

func (r *InternalResource) UnmarshalJSON(buf []byte) error {
	var raw struct {
		ID             string                 `json:"id"`
		Slug           string                 `json:"slug"`
		Name           string                 `json:"name"`
		Kind           resources.ResourceKind `json:"kind"`
		KindConfig     ResourceKindConfig     `json:"kindConfig"`
		ExportResource map[string]interface{} `json:"resource"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	var export resources.Resource
	var err error
	if raw.ExportResource != nil {
		export, err = resources.GetResource(resources.ResourceKind(raw.Kind), raw.ExportResource)
		if err != nil {
			return err
		}
	}

	r.ID = raw.ID
	r.Slug = raw.Slug
	r.Name = raw.Name
	r.Kind = raw.Kind
	r.KindConfig = raw.KindConfig
	r.ExportResource = export

	return nil
}

func (r InternalResource) ToExternalResource() (resources.Resource, error) {
	return r.KindConfig.ToExternalResource(resources.BaseResource{
		ID:   r.ID,
		Slug: r.Slug,
		Kind: r.Kind,
		Name: r.Name,
	})
}

const KindUnknown resources.ResourceKind = ""

// ResourceConfigValues should be implemented by each *KindConfig
type ResourceConfigValues interface {
	Update(v ResourceConfigValues) error
	Validate() error
	ToExternalResource(resources.BaseResource) (resources.Resource, error)
}

// ResourceKindConfig is a "union" of structs - only one of the *KindConfig values is set.
type ResourceKindConfig struct {
	BigQuery  *BigQueryKindConfig  `json:"bigquery,omitempty" yaml:"bigquery,omitempty"`
	Mailgun   *MailgunKindConfig   `json:"mailgun,omitempty" yaml:"mailgun,omitempty"`
	MongoDB   *MongoDBKindConfig   `json:"mongodb,omitempty" yaml:"mongodb,omitempty"`
	MySQL     *MySQLKindConfig     `json:"mysql,omitempty" yaml:"mysql,omitempty"`
	Postgres  *PostgresKindConfig  `json:"postgres,omitempty" yaml:"postgres,omitempty"`
	Redshift  *RedshiftKindConfig  `json:"redshift,omitempty" yaml:"redshift,omitempty"`
	REST      *RESTKindConfig      `json:"rest,omitempty" yaml:"rest,omitempty"`
	SendGrid  *SendGridKindConfig  `json:"sendgrid,omitempty" yaml:"sendgrid,omitempty"`
	Slack     *SlackKindConfig     `json:"slack,omitempty" yaml:"slack,omitempty"`
	SMTP      *SMTPKindConfig      `json:"smtp,omitempty" yaml:"smtp,omitempty"`
	Snowflake *SnowflakeKindConfig `json:"snowflake,omitempty" yaml:"snowflake,omitempty"`
	SQLServer *SQLServerKindConfig `json:"sqlserver,omitempty" yaml:"sqlserver,omitempty"`
}

func (this ResourceKindConfig) KindValue() (resources.ResourceKind, ResourceConfigValues) {
	switch {
	case this.BigQuery != nil:
		return KindBigQuery, this.BigQuery
	case this.MySQL != nil:
		return KindMySQL, this.MySQL
	case this.MongoDB != nil:
		return KindMongoDB, this.MongoDB
	case this.Postgres != nil:
		return KindPostgres, this.Postgres
	case this.Redshift != nil:
		return KindRedshift, this.Redshift
	case this.REST != nil:
		return KindREST, this.REST
	case this.Mailgun != nil:
		return KindMailgun, this.Mailgun
	case this.SendGrid != nil:
		return KindSendGrid, this.SendGrid
	case this.Slack != nil:
		return KindSlack, this.Slack
	case this.SMTP != nil:
		return KindSMTP, this.SMTP
	case this.Snowflake != nil:
		return KindSnowflake, this.Snowflake
	case this.SQLServer != nil:
		return KindSQLServer, this.SQLServer
	default:
		return KindUnknown, nil
	}
}

func (this ResourceKindConfig) Kind() resources.ResourceKind {
	kind, _ := this.KindValue()
	return kind
}

func (this *ResourceKindConfig) Config() (ResourceConfigValues, error) {
	kind, value := this.KindValue()
	if kind == KindUnknown {
		return nil, errors.New("unknown resource kind")
	}
	return value, nil
}

func (this ResourceKindConfig) Value() (driver.Value, error) {
	return json.Marshal(this)
}

func (this *ResourceKindConfig) Scan(src interface{}) error {
	source, ok := src.([]byte)
	if !ok {
		return errors.New("Type assertion .([]byte) failed")
	}
	return json.Unmarshal(source, this)
}

func (this *ResourceKindConfig) Update(c ResourceKindConfig) error {
	conf, err := this.Config()
	if err != nil {
		return err
	}
	confNew, err := c.Config()
	if err != nil {
		return err
	}
	if err := conf.Update(confNew); err != nil {
		return err
	}
	return nil
}

func (this ResourceKindConfig) Validate() error {
	conf, err := this.Config()
	if err != nil {
		return err
	}
	return conf.Validate()
}

func (this *ResourceKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	conf, err := this.Config()
	if err != nil {
		return nil, err
	}
	return conf.ToExternalResource(base)
}
