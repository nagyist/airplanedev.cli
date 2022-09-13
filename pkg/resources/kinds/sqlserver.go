package kinds

import (
	"fmt"
	"net/url"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindSQLServer resources.ResourceKind = "sqlserver"

const SQLDriverSQLServer SQLDriver = "sqlserver"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindSQLServer, func() resources.Resource { return &SQLServerResource{} })
}

type SQLServerResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	Username    string `json:"username" mapstructure:"username"`
	Host        string `json:"host" mapstructure:"host"`
	Port        string `json:"port" mapstructure:"port"`
	Database    string `json:"database" mapstructure:"database"`
	Password    string `json:"password" mapstructure:"password"`
	EncryptMode string `json:"encrypt" mapstructure:"encrypt"`
	DSN         string `json:"dsn" mapstructure:"dsn"`

	// Optional SSH tunneling
	SSHHost       string `json:"sshHost" mapstructure:"sshHost"`
	SSHPort       string `json:"sshPort" mapstructure:"sshPort"`
	SSHUsername   string `json:"sshUsername" mapstructure:"sshUsername"`
	SSHPrivateKey string `json:"sshPrivateKey" mapstructure:"sshPrivateKey"`
}

var _ SQLResourceInterface = &SQLServerResource{}

func (r *SQLServerResource) ScrubSensitiveData() {
	r.Password = ""
	r.SSHPrivateKey = ""
	r.DSN = ""
}

func (r *SQLServerResource) Update(other resources.Resource) error {
	o, ok := other.(*SQLServerResource)
	if !ok {
		return errors.Errorf("expected *SQLServerResource got %T", other)
	}

	r.Host = o.Host
	r.Port = o.Port
	r.Database = o.Database
	r.Username = o.Username
	// Don't clobber over password if empty
	if o.Password != "" {
		r.Password = o.Password
	}
	r.EncryptMode = o.EncryptMode

	r.SSHHost = o.SSHHost
	r.SSHPort = o.SSHPort
	r.SSHUsername = o.SSHUsername
	// Don't clobber over SSH private key if empty
	if o.SSHPrivateKey != "" {
		r.SSHPrivateKey = o.SSHPrivateKey
	}

	if err := r.Calculate(); err != nil {
		return errors.Wrap(err, "error computing calculated fields")
	}

	return nil
}

func (r *SQLServerResource) Calculate() error {
	r.DSN = r.dsn()
	return nil
}

func (r SQLServerResource) Validate() error {
	if r.Username == "" {
		return resources.NewErrMissingResourceField("username")
	}
	if r.Host == "" {
		return resources.NewErrMissingResourceField("host")
	}
	if r.Port == "" {
		return resources.NewErrMissingResourceField("port")
	}
	if r.Database == "" {
		return resources.NewErrMissingResourceField("database")
	}
	if r.Password == "" {
		return resources.NewErrMissingResourceField("password")
	}
	if r.EncryptMode != "disable" && r.EncryptMode != "true" {
		return errors.Errorf("Unknown encrypt string: %s", r.EncryptMode)
	}
	if r.DSN == "" {
		return resources.NewErrMissingResourceField("dsn")
	}

	if r.SSHHost != "" {
		if r.SSHPort == "" {
			return errors.New("Missing SSH port")
		}
		if r.SSHUsername == "" {
			return errors.New("Missing SSH username")
		}
		if r.SSHPrivateKey == "" {
			return errors.New("Missing SSH private key")
		}
	}

	return nil
}

func (r SQLServerResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r SQLServerResource) String() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

func (r SQLServerResource) GetDSN() string {
	return r.DSN
}

func (r SQLServerResource) GetSSHConfig() *SSHConfig {
	if r.SSHHost == "" {
		return nil
	}
	return &SSHConfig{
		Host:       r.SSHHost,
		Port:       r.SSHPort,
		Username:   r.SSHUsername,
		PrivateKey: []byte(r.SSHPrivateKey),
	}
}

func (r SQLServerResource) GetSQLDriver() SQLDriver {
	return SQLDriverSQLServer
}

func (r SQLServerResource) ID() string {
	return r.BaseResource.ID
}

func (r *SQLServerResource) UpdateBaseResource(br resources.BaseResource) error {
	r.BaseResource.Update(br)
	return nil
}

func (r SQLServerResource) dsn() string {
	q := url.Values{}
	q.Set("database", r.Database)
	q.Set("encrypt", r.EncryptMode)
	q.Set("app name", "Airplane")
	if r.EncryptMode == "true" {
		// TrustServerCertificate to match behavior of mysql (skip-verify) and postgres (require)
		// until we support configuring CA
		q.Set("TrustServerCertificate", "true")
	}

	u := url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(r.Username, r.Password),
		Host:     fmt.Sprintf("%s:%s", r.Host, r.Port),
		RawQuery: q.Encode(),
	}
	return u.String()
}
