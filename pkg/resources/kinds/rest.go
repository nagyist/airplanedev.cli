package kinds

import (
	"fmt"
	"strings"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindREST resources.ResourceKind = "rest"

func init() {
	resources.RegisterBaseResourceFactory(ResourceKindREST, func() resources.Resource { return RESTResource{} })
}

type RESTResource struct {
	resources.BaseResource `mapstructure:",squash"`

	BaseURL       string            `json:"baseURL" mapstructure:"baseURL"`
	Headers       map[string]string `json:"headers" mapstructure:"headers"`
	SecretHeaders []string          `json:"secretHeaders" mapstructure:"secretHeaders"`
}

var _ resources.Resource = RESTResource{}

func (r RESTResource) Validate() error {
	if r.BaseURL == "" {
		return resources.NewErrMissingResourceField("baseURL")
	}
	if !strings.HasPrefix(r.BaseURL, "https://") && !strings.HasPrefix(r.BaseURL, "http://") {
		return errors.Errorf("invalid URL protocol for baseURL")
	}
	for _, h := range r.SecretHeaders {
		if _, ok := r.Headers[h]; !ok {
			return errors.Errorf("%s is a secretHeader but not present in headers", h)
		}
	}

	return nil
}

func (r RESTResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r RESTResource) String() string {
	return fmt.Sprintf("RESTResource<%s>", r.BaseURL)
}

func (r RESTResource) ID() string {
	return r.BaseResource.ID
}
