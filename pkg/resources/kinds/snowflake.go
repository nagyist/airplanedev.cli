package kinds

import (
	"fmt"

	"github.com/airplanedev/lib/pkg/resources"
)

var ResourceKindSnowflake resources.ResourceKind = "snowflake"

const SQLDriverSnowflake SQLDriver = "snowflake"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindSnowflake, func() resources.Resource { return SnowflakeResource{} })
}

type SnowflakeResource struct {
	resources.BaseResource `mapstructure:",squash"`

	Account   string `json:"account" mapstructure:"account"`
	Warehouse string `json:"warehouse" mapstructure:"warehouse"`
	Database  string `json:"database" mapstructure:"database"`
	Schema    string `json:"schema" mapstructure:"schema"`
	Role      string `json:"role" mapstructure:"role"`
	Username  string `json:"username" mapstructure:"username"`
	Password  string `json:"password" mapstructure:"password"`
	DSN       string `json:"dsn" mapstructure:"dsn"`
}

var _ SQLResourceInterface = SnowflakeResource{}

func (r SnowflakeResource) Validate() error {
	if r.Account == "" {
		return resources.NewErrMissingResourceField("account")
	}
	if r.Warehouse == "" {
		return resources.NewErrMissingResourceField("warehouse")
	}
	if r.Database == "" {
		return resources.NewErrMissingResourceField("database")
	}
	if r.Username == "" {
		return resources.NewErrMissingResourceField("username")
	}
	if r.Password == "" {
		return resources.NewErrMissingResourceField("password")
	}
	if r.DSN == "" {
		return resources.NewErrMissingResourceField("dsn")
	}

	return nil
}

func (r SnowflakeResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r SnowflakeResource) String() string {
	return fmt.Sprintf("%s/%s/%s", r.Account, r.Database, r.Schema)
}

func (r SnowflakeResource) GetDSN() string {
	return r.DSN
}

func (r SnowflakeResource) GetSSHConfig() *SSHConfig {
	return nil
}

func (r SnowflakeResource) GetSQLDriver() SQLDriver {
	return SQLDriverSnowflake
}

func (r SnowflakeResource) ID() string {
	return r.BaseResource.ID
}
