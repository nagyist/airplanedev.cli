package kinds

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

var ResourceKindMongoDB resources.ResourceKind = "mongodb"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindMongoDB, func() resources.Resource { return MongoDBResource{} })
}

type MongoDBResource struct {
	resources.BaseResource `mapstructure:",squash"`

	ConnectionString string `json:"connectionString" mapstructure:"connectionString"`
}

var _ resources.Resource = MongoDBResource{}

func (r MongoDBResource) Validate() error {
	if r.ConnectionString == "" {
		return resources.NewErrMissingResourceField("connectionString")
	}
	_, err := connstring.ParseAndValidate(r.ConnectionString)
	if err != nil {
		return errors.Wrap(err, "Invalid MongoDB connection string")
	}
	return nil
}

func (r MongoDBResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r MongoDBResource) String() string {
	return "MongoDBResource"
}
