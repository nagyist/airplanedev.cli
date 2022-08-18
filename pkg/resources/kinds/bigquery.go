package kinds

import (
	"fmt"

	"github.com/airplanedev/lib/pkg/resources"
)

var ResourceKindBigQuery resources.ResourceKind = "bigquery"

const SQLDriverBigQuery SQLDriver = "bigquery"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindBigQuery, func() resources.Resource { return BigQueryResource{} })
}

type BigQueryResource struct {
	resources.BaseResource `mapstructure:",squash"`

	Credentials string `json:"credentials" mapstructure:"credentials"`
	ProjectID   string `json:"projectId" mapstructure:"projectId"`
	Location    string `json:"location" mapstructure:"location"`
	DataSet     string `json:"dataSet" mapstructure:"dataSet"`
	DSN         string `json:"dsn" mapstructure:"dsn"`
}

var _ SQLResourceInterface = BigQueryResource{}

func (r BigQueryResource) Validate() error {
	if r.Credentials == "" {
		return resources.NewErrMissingResourceField("credentials")
	}
	if r.Location == "" {
		return resources.NewErrMissingResourceField("location")
	}
	if r.DataSet == "" {
		return resources.NewErrMissingResourceField("dataSet")
	}
	if r.ProjectID == "" {
		return resources.NewErrMissingResourceField("projectId")
	}
	if r.DSN == "" {
		return resources.NewErrMissingResourceField("dsn")
	}

	return nil
}

func (r BigQueryResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r BigQueryResource) String() string {
	return fmt.Sprintf("%s/%s/%s", r.ProjectID, r.Location, r.DataSet)
}

func (r BigQueryResource) GetDSN() string {
	return r.DSN
}

func (r BigQueryResource) GetSSHConfig() *SSHConfig {
	return nil
}

func (r BigQueryResource) GetSQLDriver() SQLDriver {
	return SQLDriverBigQuery
}

func (r BigQueryResource) ID() string {
	return r.BaseResource.ID
}
