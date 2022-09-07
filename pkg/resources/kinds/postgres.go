package kinds

import (
	"fmt"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindPostgres resources.ResourceKind = "postgres"

const SQLDriverPostgres SQLDriver = "postgres"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindPostgres, func() resources.Resource { return &PostgresResource{} })
}

type PostgresResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	Username string `json:"username" mapstructure:"username"`
	Host     string `json:"host" mapstructure:"host"`
	Port     string `json:"port" mapstructure:"port"`
	Database string `json:"database" mapstructure:"database"`
	Password string `json:"password" mapstructure:"password"`
	SSLMode  string `json:"ssl" mapstructure:"ssl"`
	DSN      string `json:"dsn" mapstructure:"dsn"`

	// Optional SSH tunneling
	SSHHost       string `json:"sshHost" mapstructure:"sshHost"`
	SSHPort       string `json:"sshPort" mapstructure:"sshPort"`
	SSHUsername   string `json:"sshUsername" mapstructure:"sshUsername"`
	SSHPrivateKey string `json:"sshPrivateKey" mapstructure:"sshPrivateKey"`
}

var _ SQLResourceInterface = &PostgresResource{}

func (r *PostgresResource) ScrubSensitiveData() {
	r.Password = ""
	r.SSHPrivateKey = ""
	r.DSN = ""
}

func (r PostgresResource) Validate() error {
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
	if r.SSLMode != "disable" && r.SSLMode != "require" {
		return errors.Errorf("Unknown SSLMode: %s", r.SSLMode)
	}
	if r.DSN == "" {
		return resources.NewErrMissingResourceField("dsn")
	}

	if r.SSHHost != "" {
		if r.SSHPort == "" {
			return resources.NewErrMissingResourceField("sshPort")
		}
		if r.SSHUsername == "" {
			return resources.NewErrMissingResourceField("sshUsername")
		}
		if r.SSHPrivateKey == "" {
			return resources.NewErrMissingResourceField("sshPrivateKey")
		}
	}

	return nil
}

func (r PostgresResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r PostgresResource) String() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

func (r PostgresResource) GetDSN() string {
	return r.DSN
}

func (r PostgresResource) GetSSHConfig() *SSHConfig {
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

func (r PostgresResource) GetSQLDriver() SQLDriver {
	return SQLDriverPostgres
}

func (r PostgresResource) ID() string {
	return r.BaseResource.ID
}
