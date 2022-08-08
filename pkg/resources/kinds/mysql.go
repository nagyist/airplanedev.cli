package kinds

import (
	"fmt"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

var ResourceKindMySQL resources.ResourceKind = "mysql"

const SQLDriverMySQL SQLDriver = "mysql"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindMySQL, func() resources.Resource { return MySQLResource{} })
}

type MySQLResource struct {
	resources.BaseResource `mapstructure:",squash"`

	Username string `json:"username" mapstructure:"username"`
	Host     string `json:"host" mapstructure:"host"`
	Port     string `json:"port" mapstructure:"port"`
	Database string `json:"database" mapstructure:"database"`
	Password string `json:"password" mapstructure:"password"`
	TLS      string `json:"tls" mapstructure:"tls"`
	DSN      string `json:"dsn" mapstructure:"dsn"`

	// Optional SSH tunneling
	SSHHost       string `json:"sshHost" mapstructure:"sshHost"`
	SSHPort       string `json:"sshPort" mapstructure:"sshPort"`
	SSHUsername   string `json:"sshUsername" mapstructure:"sshUsername"`
	SSHPrivateKey string `json:"sshPrivateKey" mapstructure:"sshPrivateKey"`
}

var _ SQLResourceInterface = MySQLResource{}

func (r MySQLResource) Validate() error {
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
	if r.TLS != "false" && r.TLS != "skip-verify" {
		return errors.Errorf("Unknown TLS string: %s", r.TLS)
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

func (r MySQLResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r MySQLResource) String() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

func (r MySQLResource) GetDSN() string {
	return dsnForMySQL(r.Username, r.Host, r.Port, r.Database, r.TLS, r.Password)
}

func (r MySQLResource) GetSSHConfig() *SSHConfig {
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

func (r MySQLResource) GetSQLDriver() SQLDriver {
	return SQLDriverMySQL
}

func dsnForMySQL(username, host, port, database, tls, password string) string {
	cfg := mysql.NewConfig()
	cfg.User = username
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%s", host, port)
	cfg.DBName = database
	cfg.TLSConfig = tls
	cfg.Passwd = password
	return cfg.FormatDSN()
}
