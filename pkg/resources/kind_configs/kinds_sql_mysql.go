package kind_configs

import (
	"fmt"
	"net/url"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

const (
	KindMySQL ResourceKind = "mysql"
)

func init() {
	ResourceKindToKindConfig[KindMySQL] = &MySQLKindConfig{}
}

type MySQLKindConfig struct {
	SqlBaseConfig `yaml:",inline"`
}

func (this *MySQLKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*MySQLKindConfig)
	if !ok {
		return errors.Errorf("expected *MySQLKindConfig got %T", cv)
	}
	this.SqlBaseConfig.update(c.SqlBaseConfig)
	return nil
}

func (this MySQLKindConfig) Validate() error {
	r, err := this.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func (this MySQLKindConfig) tlsString() string {
	if this.DisableSSL {
		return "false"
	} else {
		return "skip-verify"
	}
}

func (this MySQLKindConfig) dsn() string {
	q := url.Values{}
	q.Set("tls", this.tlsString())
	u := url.URL{
		Scheme:   "mysql",
		User:     url.UserPassword(this.Username, this.Password),
		Host:     fmt.Sprintf("%s:%s", this.Host, this.Port),
		Path:     this.Database,
		RawQuery: q.Encode(),
	}
	return u.String()
}

func (this MySQLKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	return kinds.MySQLResource{
		BaseResource: base,
		Username:     this.Username,
		Host:         this.Host,
		Port:         this.Port,
		Database:     this.Database,
		TLS:          this.tlsString(),
		Password:     this.Password,
		DSN:          this.dsn(),

		SSHHost:       this.SSHHost,
		SSHPort:       this.SSHPort,
		SSHUsername:   this.SSHUsername,
		SSHPrivateKey: this.SSHPrivateKey,
	}, nil
}
