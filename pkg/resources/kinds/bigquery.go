package kinds

import (
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindBigQuery resources.ResourceKind = "bigquery"

const SQLDriverBigQuery SQLDriver = "bigquery"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindBigQuery, func() resources.Resource { return &BigQueryResource{} })
}

type BigQueryResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	Credentials string `json:"credentials" mapstructure:"credentials"`
	ProjectID   string `json:"projectId" mapstructure:"projectId"`
	Location    string `json:"location" mapstructure:"location"`
	DataSet     string `json:"dataSet" mapstructure:"dataSet"`
	DSN         string `json:"dsn" mapstructure:"dsn"`
}

var _ SQLResourceInterface = &BigQueryResource{}

func (r *BigQueryResource) ScrubSensitiveData() {
	r.Credentials = ""
	r.DSN = ""
}

func (r *BigQueryResource) Update(other resources.Resource) error {
	o, ok := other.(*BigQueryResource)
	if !ok {
		return errors.Errorf("expected *BigQueryResource got %T", other)
	}

	r.ProjectID = o.ProjectID
	r.Location = o.Location
	r.DataSet = o.DataSet
	// Don't clobber over credentials if empty
	if o.Credentials != "" {
		// BigQuery creds are in json, but
		// driver requires creds to be in base64
		creds := base64.StdEncoding.EncodeToString([]byte(o.Credentials))
		r.Credentials = creds
	}

	r.DSN = r.dsn()

	return nil
}

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

func (r *BigQueryResource) UpdateBaseResource(br resources.BaseResource) error {
	r.BaseResource.Update(br)
	return nil
}

func (r BigQueryResource) dsn() string {
	query := url.Values{}
	query.Set("credentials", r.Credentials)

	u := &url.URL{
		Scheme:   "bigquery",
		Path:     fmt.Sprintf("%s/%s/%s", r.ProjectID, r.Location, r.DataSet),
		RawQuery: query.Encode(),
	}
	return u.String()
}
