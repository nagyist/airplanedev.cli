package kinds

import (
	"fmt"

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
