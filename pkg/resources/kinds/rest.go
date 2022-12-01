package kinds

import (
	"encoding/base64"
	"fmt"
	"reflect"
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
	Headers       map[string]string `json:"headers,omitempty" mapstructure:"headers"`
	SecretHeaders []string          `json:"secretHeaders,omitempty" mapstructure:"secretHeaders"`
	Auth          RESTAuth          `json:"auth,omitempty" mapstructure:"-"`
}

var _ resources.Resource = &RESTResource{}

type RESTAuth interface {
	scrubSensitiveData()
	update(a RESTAuth) error
	calculate() error
	scrubCalculatedFields()
}

type RESTAuthKind string

const (
	RESTAuthKindBasic RESTAuthKind = "basic"
)

type RESTAuthBasic struct {
	Kind     RESTAuthKind      `json:"kind" mapstructure:"kind"`
	Username *string           `json:"username,omitempty" mapstructure:"username,omitempty"`
	Password *string           `json:"password,omitempty" mapstructure:"password,omitempty"`
	Headers  map[string]string `json:"headers,omitempty" mapstructure:"headers"`
}

func RESTResourceFactory(serialized map[string]interface{}) (resources.Resource, error) {
	resource := RESTResource{}

	serializedAuth, ok := serialized["auth"]
	if ok && serializedAuth != nil {
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
			resource.Auth = &RESTAuthBasic{}
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
	return &resource, nil
}

func (r *RESTResource) ScrubSensitiveData() {
	scrubbedHeaders := map[string]string{}
	for k, v := range r.Headers {
		if isSecretHeader(r.SecretHeaders, k) {
			scrubbedHeaders[k] = ""
		} else {
			scrubbedHeaders[k] = v
		}
	}
	r.Headers = scrubbedHeaders

	if r.Auth != nil {
		r.Auth.scrubSensitiveData()
	}
}

func (r *RESTResource) Update(other resources.Resource) error {
	o, ok := other.(*RESTResource)
	if !ok {
		return errors.Errorf("expected *RESTResource got %T", other)
	}
	r.BaseURL = o.BaseURL

	r.SecretHeaders = o.SecretHeaders
	// Copy all new headers but use existing value if it's a secret and empty.
	updatedHeaders := map[string]string{}
	for k, v := range o.Headers {
		if isSecretHeader(o.SecretHeaders, k) && v == "" {
			updatedHeaders[k] = r.Headers[k]
		} else {
			updatedHeaders[k] = v
		}
	}
	r.Headers = updatedHeaders

	if r.Auth != nil && o.Auth != nil && reflect.TypeOf(r.Auth) == reflect.TypeOf(o.Auth) {
		if err := r.Auth.update(o.Auth); err != nil {
			return err
		}
	} else {
		r.Auth = o.Auth
	}

	if err := r.Calculate(); err != nil {
		return errors.Wrap(err, "error computing calculated fields")
	}

	return nil
}

func (r *RESTResource) Calculate() error {
	if r.Auth != nil {
		if err := r.Auth.calculate(); err != nil {
			return errors.Wrap(err, "error calculating fields on REST auth")
		}
	}
	return nil
}

func (r *RESTResource) ScrubCalculatedFields() {
	if r.Auth != nil {
		r.Auth.scrubCalculatedFields()
	}
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

func (r *RESTResource) UpdateBaseResource(br resources.BaseResource) error {
	r.BaseResource.Update(br)
	return nil
}

func isSecretHeader(secretHeaders []string, header string) bool {
	for _, secretHeader := range secretHeaders {
		if secretHeader == header {
			return true
		}
	}
	return false
}

func (a *RESTAuthBasic) scrubSensitiveData() {
	a.Username = nil
	a.Password = nil
	a.Headers = map[string]string{}
}

func (a *RESTAuthBasic) update(other RESTAuth) error {
	o, ok := other.(*RESTAuthBasic)
	if !ok {
		return errors.Errorf("expected *RESTAuthBasic got %T", other)
	}

	// nil in the update means don't overwrite the username
	if o.Username != nil {
		a.Username = o.Username
	}
	// nil in the update means don't overwrite the password.
	if o.Password != nil {
		a.Password = o.Password
	}

	if err := a.calculate(); err != nil {
		return errors.Wrap(err, "error computing calculated fields on RESTAuthBasic")
	}

	return nil
}

func (a *RESTAuthBasic) calculate() error {
	credentials := fmt.Sprintf("%s:%s", *a.Username, *a.Password)
	token := base64.StdEncoding.EncodeToString([]byte(credentials))
	a.Headers = map[string]string{
		"Authorization": fmt.Sprintf("Basic %s", token),
	}
	return nil
}

func (a *RESTAuthBasic) scrubCalculatedFields() {
	a.Headers = map[string]string{}
}
