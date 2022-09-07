package kinds

import (
	"fmt"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

var ResourceKindSMTP resources.ResourceKind = "smtp"

func init() {
	resources.RegisterResourceFactory(ResourceKindSMTP, SMTPResourceFactory)
}

type SMTPResource struct {
	resources.BaseResource `mapstructure:",squash" yaml:",inline"`

	Hostname string   `json:"hostname" mapstructure:"hostname"`
	Port     string   `json:"port" mapstructure:"port"`
	Auth     SMTPAuth `json:"auth" mapstructure:"-"`
}

var _ resources.Resource = &SMTPResource{}

type SMTPAuth interface {
	scrubSensitiveData()
	validate(path string) error
}

type EmailSMTPAuthKind string

const (
	EmailSMTPAuthKindPlain   EmailSMTPAuthKind = "plain"
	EmailSMTPAuthKindCRAMMD5 EmailSMTPAuthKind = "crammd5"
	EmailSMTPAuthKindLogin   EmailSMTPAuthKind = "login"
)

type SMTPAuthPlain struct {
	Kind     EmailSMTPAuthKind `json:"kind" mapstructure:"kind"`
	Identity string            `json:"identity" mapstructure:"identity"`
	Username string            `json:"username" mapstructure:"username"`
	Password string            `json:"password" mapstructure:"password"`
}

type SMTPAuthCRAMMD5 struct {
	Kind     EmailSMTPAuthKind `json:"kind" mapstructure:"kind"`
	Username string            `json:"username" mapstructure:"username"`
	Secret   string            `json:"secret" mapstructure:"secret"`
}

type SMTPAuthLogin struct {
	Kind     EmailSMTPAuthKind `json:"kind" mapstructure:"kind"`
	Username string            `json:"username" mapstructure:"username"`
	Password string            `json:"password" mapstructure:"password"`
}

func SMTPResourceFactory(serialized map[string]interface{}) (resources.Resource, error) {
	resource := &SMTPResource{}

	serializedAuth, ok := serialized["auth"]
	if ok {
		authMap, ok := serializedAuth.(map[string]interface{})
		if !ok {
			return nil, errors.Errorf("expected auth to be a map, got %T", serializedAuth)
		}

		kind, ok := authMap["kind"]
		if !ok {
			return nil, errors.New("missing kind property on SMTP auth")
		}

		kindStr, ok := kind.(string)
		if !ok {
			return nil, errors.Errorf("expected kind to be a string, got %T", kind)
		}

		switch kindStr {
		case "plain":
			resource.Auth = &SMTPAuthPlain{}
			if err := resources.BaseFactory(authMap, &resource.Auth); err != nil {
				return nil, err
			}
		case "crammd5":
			resource.Auth = &SMTPAuthCRAMMD5{}
			if err := resources.BaseFactory(authMap, &resource.Auth); err != nil {
				return nil, err
			}
		case "login":
			resource.Auth = &SMTPAuthLogin{}
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

func (r *SMTPResource) ScrubSensitiveData() {
	if r.Auth != nil {
		r.Auth.scrubSensitiveData()
	}
}

func (r SMTPResource) Validate() error {
	if r.Hostname == "" {
		return resources.NewErrMissingResourceField("hostname")
	}
	if r.Port == "" {
		return resources.NewErrMissingResourceField("port")
	}
	if r.Auth != nil {
		if err := r.Auth.validate("auth"); err != nil {
			return err
		}
	} else {
		return resources.NewErrMissingResourceField("auth")
	}

	return nil
}

func (r SMTPResource) Kind() resources.ResourceKind {
	return r.BaseResource.Kind
}

func (r SMTPResource) String() string {
	return fmt.Sprintf("SMTPResource<%s:%s>", r.Hostname, r.Port)
}

func (r SMTPResource) ID() string {
	return r.BaseResource.ID
}

func (a *SMTPAuthPlain) scrubSensitiveData() {
	a.Password = ""
}

func (a *SMTPAuthPlain) validate(path string) error {
	// Identity can & usually is empty string, no need to check.
	if a.Username == "" {
		return resources.NewErrMissingResourceField(fmt.Sprintf("%s.username", path))
	}
	if a.Password == "" {
		return resources.NewErrMissingResourceField(fmt.Sprintf("%s.password", path))
	}
	return nil
}

func (a *SMTPAuthCRAMMD5) scrubSensitiveData() {
	a.Secret = ""
}

func (a *SMTPAuthCRAMMD5) validate(path string) error {
	if a.Username == "" {
		return resources.NewErrMissingResourceField(fmt.Sprintf("%s.username", path))
	}
	if a.Secret == "" {
		return resources.NewErrMissingResourceField(fmt.Sprintf("%s.secret", path))
	}
	return nil
}

func (a *SMTPAuthLogin) scrubSensitiveData() {
	a.Password = ""
}

func (a *SMTPAuthLogin) validate(path string) error {
	if a.Username == "" {
		return resources.NewErrMissingResourceField(fmt.Sprintf("%s.username", path))
	}
	if a.Password == "" {
		return resources.NewErrMissingResourceField(fmt.Sprintf("%s.password", path))
	}
	return nil
}
