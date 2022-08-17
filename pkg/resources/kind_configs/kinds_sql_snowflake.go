package kind_configs

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
	"github.com/snowflakedb/gosnowflake"
)

const (
	KindSnowflake ResourceKind = "snowflake"
)

func init() {
	ResourceKindToKindConfig[KindSnowflake] = &SnowflakeKindConfig{}
}

type SnowflakeKindConfig struct {
	Account   string `json:"account" yaml:"account"`
	Warehouse string `json:"warehouse" yaml:"warehouse"`
	Database  string `json:"database" yaml:"database"`
	Schema    string `json:"schema" yaml:"schema"`
	Role      string `json:"role" yaml:"role"`
	Username  string `json:"username" yaml:"username"`
	Password  string `json:"password" yaml:"password"`
}

func (this *SnowflakeKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*SnowflakeKindConfig)
	if !ok {
		return errors.Errorf("expected *SnowflakeKindConfig got %T", cv)
	}
	this.Account = c.Account
	this.Warehouse = c.Warehouse
	this.Database = c.Database
	this.Schema = c.Schema
	this.Role = c.Role
	this.Username = c.Username
	// Don't clobber over password if empty
	if c.Password != "" {
		this.Password = c.Password
	}
	return nil
}

func (this SnowflakeKindConfig) Validate() error {
	r, err := this.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func (this SnowflakeKindConfig) dsn() (string, error) {
	cfg := gosnowflake.Config{
		Account:     this.Account,
		Warehouse:   this.Warehouse,
		Database:    this.Database,
		Schema:      this.Schema,
		Role:        this.Role,
		User:        this.Username,
		Password:    this.Password,
		Application: "Airplane",
	}
	return gosnowflake.DSN(&cfg)
}

func (this SnowflakeKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	dsn, err := this.dsn()
	if err != nil {
		return nil, err
	}

	return kinds.SnowflakeResource{
		BaseResource: base,
		Username:     this.Username,
		Database:     this.Database,
		Password:     this.Password,
		DSN:          dsn,
		Account:      this.Account,
		Warehouse:    this.Warehouse,
		Schema:       this.Schema,
		Role:         this.Role,
	}, nil
}
