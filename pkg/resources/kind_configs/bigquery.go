package kind_configs

import (
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

const (
	KindBigQuery resources.ResourceKind = "bigquery"
)

func init() {
	ResourceKindToKindConfig[KindBigQuery] = &BigQueryKindConfig{}
}

type BigQueryKindConfig struct {
	Credentials string `json:"credentials" yaml:"credentials"`
	ProjectID   string `json:"projectId" yaml:"projectId"`
	Location    string `json:"location" yaml:"location"`
	DataSet     string `json:"dataSet" yaml:"dataSet"`
}

func (this *BigQueryKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*BigQueryKindConfig)
	if !ok {
		return errors.Errorf("expected *BigQueryKindConfig got %T", cv)
	}
	this.ProjectID = c.ProjectID
	this.Location = c.Location
	this.DataSet = c.DataSet
	// Don't clobber over credentials if empty
	if c.Credentials != "" {
		// BigQuery creds are in json, but
		// driver requires creds to be in base64
		creds := base64.StdEncoding.EncodeToString([]byte(c.Credentials))
		this.Credentials = creds
	}
	return nil
}

func (this BigQueryKindConfig) Validate() error {
	r, err := this.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func (this BigQueryKindConfig) dsn() (string, error) {
	query := url.Values{}
	query.Set("credentials", this.Credentials)

	u := &url.URL{
		Scheme:   "bigquery",
		Path:     fmt.Sprintf("%s/%s/%s", this.ProjectID, this.Location, this.DataSet),
		RawQuery: query.Encode(),
	}
	return u.String(), nil
}

func (this BigQueryKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	dsn, err := this.dsn()
	if err != nil {
		return nil, err
	}

	return &kinds.BigQueryResource{
		BaseResource: base,
		Credentials:  this.Credentials,
		ProjectID:    this.ProjectID,
		Location:     this.Location,
		DataSet:      this.DataSet,
		DSN:          dsn,
	}, nil
}
