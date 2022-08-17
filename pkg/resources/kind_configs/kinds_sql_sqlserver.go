package kind_configs

import (
	"fmt"
	"net/url"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

const (
	KindSQLServer ResourceKind = "sqlserver"
)

func init() {
	ResourceKindToKindConfig[KindSQLServer] = &SQLServerKindConfig{}
}

type SQLServerKindConfig struct {
	SqlBaseConfig `yaml:",inline"`
}

func (this *SQLServerKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*SQLServerKindConfig)
	if !ok {
		return errors.Errorf("expected *SQLServerKindConfig got %T", cv)
	}
	this.SqlBaseConfig.update(c.SqlBaseConfig)
	return nil
}

func (this SQLServerKindConfig) Validate() error {
	r, err := this.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func (this SQLServerKindConfig) encryptString() string {
	if this.DisableSSL {
		return "disable"
	} else {
		return "true"
	}
}

func (this SQLServerKindConfig) dsn() string {
	q := url.Values{}
	q.Set("database", this.Database)
	q.Set("encrypt", this.encryptString())
	q.Set("app name", "Airplane")
	if !this.DisableSSL {
		// TrustServerCertificate to match behavior of mysql (skip-verify) and postgres (require)
		// until we support configuring CA
		q.Set("TrustServerCertificate", "true")
	}

	u := url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(this.Username, this.Password),
		Host:     fmt.Sprintf("%s:%s", this.Host, this.Port),
		RawQuery: q.Encode(),
	}
	return u.String()
}

func (this SQLServerKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	return kinds.SQLServerResource{
		BaseResource: base,
		Username:     this.Username,
		Host:         this.Host,
		Port:         this.Port,
		Database:     this.Database,
		EncryptMode:  this.encryptString(),
		Password:     this.Password,
		DSN:          this.dsn(),

		SSHHost:       this.SSHHost,
		SSHPort:       this.SSHPort,
		SSHUsername:   this.SSHUsername,
		SSHPrivateKey: this.SSHPrivateKey,
	}, nil
}
