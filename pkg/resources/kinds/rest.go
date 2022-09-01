package kinds

import (
	"fmt"
	"strings"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindREST resources.ResourceKind = "rest"

func init() {
	resources.RegisterResourceFactory(ResourceKindREST, RESTResourceFactory)
}

type RESTResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	BaseURL       string            `json:"baseURL" mapstructure:"baseURL"`
	Headers       map[string]string `json:"headers" mapstructure:"headers"`
	SecretHeaders []string          `json:"secretHeaders" mapstructure:"secretHeaders"`
	Auth          RESTAuth          `json:"auth" mapstructure:"-"`
}

var _ resources.Resource = RESTResource{}

type RESTAuth interface {
}

type RESTAuthKind string

const (
	RESTAuthKindBasic RESTAuthKind = "basic"
)

type RESTAuthBasic struct {
	Kind     RESTAuthKind      `json:"kind" mapstructure:"kind"`
	Username *string           `json:"username,omitempty" mapstructure:"username,omitempty"`
	Password *string           `json:"password,omitempty" mapstructure:"password,omitempty"`
	Headers  map[string]string `json:"headers" mapstructure:"headers"`
}

func RESTResourceFactory(serialized map[string]interface{}) (resources.Resource, error) {
	resource := RESTResource{}

	serializedAuth, ok := serialized["auth"]
	if ok {
		authMap, ok := serializedAuth.(map[string]interface{})
		if !ok {
			return nil, errors.Errorf("expected auth to be a map, got %T", serializedAuth)
		}

		kind, ok := authMap["kind"]
		if !ok {
			return nil, errors.New("missing kind property on REST auth")
		}

		kindStr, ok := kind.(string)
		if !ok {
			return nil, errors.Errorf("expected kind to be a string, got %T", kind)
		}

		switch kindStr {
		case string(RESTAuthKindBasic):
			resource.Auth = RESTAuthBasic{}
			if err := resources.BaseFactory(authMap, &resource.Auth); err != nil {
				return nil, err
			}
		default:
			return nil, errors.Errorf("unsupported auth kind: %s", kindStr)
		}
	}

	if err := resources.BaseFactory(serialized, &resource); err != nil {
		return nil, err
	}
	return resource, nil
}

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
