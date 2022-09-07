package kinds

import (
	"fmt"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
	"github.com/snowflakedb/gosnowflake"
)

var ResourceKindSnowflake resources.ResourceKind = "snowflake"

const SQLDriverSnowflake SQLDriver = "snowflake"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindSnowflake, func() resources.Resource { return &SnowflakeResource{} })
}

type SnowflakeResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	Account   string `json:"account" mapstructure:"account"`
	Warehouse string `json:"warehouse" mapstructure:"warehouse"`
	Database  string `json:"database" mapstructure:"database"`
	Schema    string `json:"schema" mapstructure:"schema"`
	Role      string `json:"role" mapstructure:"role"`
	Username  string `json:"username" mapstructure:"username"`
	Password  string `json:"password" mapstructure:"password"`
	DSN       string `json:"dsn" mapstructure:"dsn"`
}

var _ SQLResourceInterface = &SnowflakeResource{}

func (r *SnowflakeResource) ScrubSensitiveData() {
	r.Password = ""
	r.DSN = ""
}

func (r *SnowflakeResource) Update(other resources.Resource) error {
	o, ok := other.(*SnowflakeResource)
	if !ok {
		return errors.Errorf("expected *SnowflakeResource got %T", other)
	}

	r.Account = o.Account
	r.Warehouse = o.Warehouse
	r.Database = o.Database
	r.Schema = o.Schema
	r.Role = o.Role
	r.Username = o.Username
	// Don't clobber over password if empty
	if o.Password != "" {
		r.Password = o.Password
	}

	dsn, err := r.dsn()
	if err != nil {
		return errors.Wrapf(err, "error calculating DSN")
	}
	r.DSN = dsn

	return nil
}

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

func (r SnowflakeResource) dsn() (string, error) {
	cfg := gosnowflake.Config{
		Account:     r.Account,
		Warehouse:   r.Warehouse,
		Database:    r.Database,
		Schema:      r.Schema,
		Role:        r.Role,
		User:        r.Username,
		Password:    r.Password,
		Application: "Airplane",
	}
	return gosnowflake.DSN(&cfg)
}
