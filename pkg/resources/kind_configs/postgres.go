package kind_configs

import (
	"fmt"
	"net/url"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

const (
	KindPostgres resources.ResourceKind = "postgres"
)

func init() {
	ResourceKindToKindConfig[KindPostgres] = &PostgresKindConfig{}
}

type PostgresKindConfig struct {
	SqlBaseConfig `yaml:",inline"`
}

type NewPostgresConfigOptions struct {
	Host       string
	Port       string
	Database   string
	Username   string
	Password   string
	DisableSSL bool
}

func NewPostgresKindConfig(c NewPostgresConfigOptions) *PostgresKindConfig {
	return &PostgresKindConfig{
		SqlBaseConfig{
			Host:       c.Host,
			Port:       c.Port,
			Database:   c.Database,
			Username:   c.Username,
			Password:   c.Password,
			DisableSSL: c.DisableSSL,
		},
	}
}

var _ ResourceConfigValues = &PostgresKindConfig{}

func (this *PostgresKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*PostgresKindConfig)
	if !ok {
		return errors.Errorf("expected *PostgresKindConfig got %T", cv)
	}
	this.SqlBaseConfig.update(c.SqlBaseConfig)
	return nil
}

func (this PostgresKindConfig) Validate() error {
	r, err := this.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func (this PostgresKindConfig) sslModeString() string {
	// Currently only support disable/require - could potentially also check verify-full if we
	// support configuring CAs.
	if this.DisableSSL {
		return "disable"
	} else {
		return "require"
	}
}

func (this PostgresKindConfig) dsn() string {
	q := url.Values{}
	q.Set("sslmode", this.sslModeString())
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(this.Username, this.Password),
		Host:     fmt.Sprintf("%s:%s", this.Host, this.Port),
		Path:     this.Database,
		RawQuery: q.Encode(),
	}
	return u.String()
}

func (this PostgresKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	return &kinds.PostgresResource{
		BaseResource: base,
		Username:     this.Username,
		Host:         this.Host,
		Port:         this.Port,
		Database:     this.Database,
		SSLMode:      this.sslModeString(),
		Password:     this.Password,
		DSN:          this.dsn(),

		SSHHost:       this.SSHHost,
		SSHPort:       this.SSHPort,
		SSHUsername:   this.SSHUsername,
		SSHPrivateKey: this.SSHPrivateKey,
	}, nil
}
