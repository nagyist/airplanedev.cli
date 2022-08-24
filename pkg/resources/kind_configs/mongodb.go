package kind_configs

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

const (
	KindMongoDB resources.ResourceKind = "mongodb"
)

func init() {
	ResourceKindToKindConfig[KindMongoDB] = &MongoDBKindConfig{}
}

type MongoDBKindConfig struct {
	ConnectionString string `json:"connectionString" yaml:"connectionString"`
}

var _ ResourceConfigValues = &MongoDBKindConfig{}

func (this *MongoDBKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*MongoDBKindConfig)
	if !ok {
		return errors.Errorf("expected *MongoDBKindConfig, got %T", cv)
	}
	// Only update connection string if it's not empty
	if c.ConnectionString != "" {
		this.ConnectionString = c.ConnectionString
	}
	return nil
}

func (this MongoDBKindConfig) Validate() error {
	r, err := this.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func (this MongoDBKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	return kinds.MongoDBResource{
		BaseResource:     base,
		ConnectionString: this.ConnectionString,
	}, nil
}
