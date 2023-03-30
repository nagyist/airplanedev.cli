package kinds

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

var ResourceKindMongoDB resources.ResourceKind = "mongodb"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindMongoDB, func() resources.Resource { return &MongoDBResource{} })
}

type MongoDBResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	ConnectionString string `json:"connectionString" mapstructure:"connectionString"`
}

var _ resources.Resource = &MongoDBResource{}

func (r *MongoDBResource) ScrubSensitiveData() {
	r.ConnectionString = ""
}

func (r *MongoDBResource) Update(other resources.Resource) error {
	o, ok := other.(*MongoDBResource)
	if !ok {
		return errors.Errorf("expected *MongoDBResource got %T", other)
	}

	// Only update connection string if it's not empty
	if o.ConnectionString != "" {
		r.ConnectionString = o.ConnectionString
	}

	if err := r.Calculate(); err != nil {
		return errors.Wrap(err, "error computing calculated fields")
	}

	return nil
}

func (r *MongoDBResource) Calculate() error {
	return nil
}

func (r *MongoDBResource) ScrubCalculatedFields() {}

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

func (r MongoDBResource) ID() string {
	return r.BaseResource.ID
}

func (r *MongoDBResource) UpdateBaseResource(br resources.BaseResource) error {
	r.BaseResource.Update(br)
	return nil
}
