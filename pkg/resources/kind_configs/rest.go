package kind_configs

import (
	"encoding/base64"
	"fmt"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

const KindREST resources.ResourceKind = "rest"

func init() {
	ResourceKindToKindConfig[KindREST] = &RESTKindConfig{}
}

type RESTKindConfig struct {
	BaseURL       string            `json:"baseURL" yaml:"baseURL"`
	Headers       map[string]string `json:"headers" yaml:"headers"`
	SecretHeaders []string          `json:"secretHeaders,omitempty" yaml:"secretHeaders"`
	AuthConfig    *RESTAuthConfig   `json:"authConfig,omitempty" yaml:"authConfig"`
}

var _ ResourceConfigValues = &RESTKindConfig{}

func (this *RESTKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*RESTKindConfig)
	if !ok {
		return errors.Errorf("expected *RESTKindConfig got %T", cv)
	}
	this.BaseURL = c.BaseURL

	this.SecretHeaders = c.SecretHeaders
	// Copy all new headers but use existing value if it's a secret and empty.
	updatedHeaders := map[string]string{}
	for k, v := range c.Headers {
		if isSecretHeader(c.SecretHeaders, k) && v == "" {
			updatedHeaders[k] = this.Headers[k]
		} else {
			updatedHeaders[k] = v
		}
	}
	this.Headers = updatedHeaders

	if this.AuthConfig != nil && c.AuthConfig != nil {
		this.AuthConfig.update(c.AuthConfig)
	} else {
		this.AuthConfig = c.AuthConfig
	}
	return nil
}

func (this RESTKindConfig) Validate() error {
	if this.AuthConfig != nil {
		if err := this.AuthConfig.validate(); err != nil {
			return err
		}
	}
	r, err := this.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func isSecretHeader(secretHeaders []string, header string) bool {
	for _, secretHeader := range secretHeaders {
		if secretHeader == header {
			return true
		}
	}
	return false
}

func (this RESTKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	r := kinds.RESTResource{
		BaseResource:  base,
		BaseURL:       this.BaseURL,
		SecretHeaders: this.SecretHeaders,
	}

	r.Headers = map[string]string{}
	if this.Headers != nil {
		for k, v := range this.Headers {
			r.Headers[k] = v
		}
	}

	if this.AuthConfig != nil {
		auth, err := this.AuthConfig.toExternalAuth()
		if err != nil {
			return nil, errors.Wrap(err, "converting AuthConfig to external auth")
		}
		r.Auth = auth
	}

	return &r, nil
}

type RESTAuthConfig struct {
	Kind     RESTAuthConfigKind `json:"kind"`
	Username *string            `json:"username,omitempty"`
	Password *string            `json:"password,omitempty"`
}

type RESTAuthConfigKind string

const (
	KindBasic RESTAuthConfigKind = "basic"
)

func (this *RESTAuthConfig) update(c *RESTAuthConfig) {
	if this.Kind != c.Kind {
		this.reset()
	}

	this.Kind = c.Kind
	switch this.Kind {
	case KindBasic:
		// nil in the update means don't overwrite the username
		if c.Username != nil {
			this.Username = c.Username
		}
		// nil in the update means don't overwrite the password.
		if c.Password != nil {
			this.Password = c.Password
		}
	}
}

func (this *RESTAuthConfig) reset() {
	this.Username = nil
	this.Password = nil
}

func (this RESTAuthConfig) validate() error {
	switch this.Kind {
	case KindBasic:
		if this.Username == nil || *this.Username == "" {
			return errors.New("missing username for basic auth")
		}
		if this.Password == nil {
			return errors.New("missing password for basic auth")
		}
		return nil
	default:
		return errors.Errorf("unknown auth kind: %s", this.Kind)
	}
}

func (this RESTAuthConfig) authHeader() (string, error) {
	switch this.Kind {
	case KindBasic:
		credentials := fmt.Sprintf("%s:%s", *this.Username, *this.Password)
		token := base64.StdEncoding.EncodeToString([]byte(credentials))
		return fmt.Sprintf("Basic %s", token), nil
	default:
		return "", errors.Errorf("unknown auth kind: %s", this.Kind)
	}
}

func (this RESTAuthConfig) toExternalAuth() (kinds.RESTAuth, error) {
	switch this.Kind {
	case KindBasic:
		header, err := this.authHeader()
		if err != nil {
			return nil, errors.Wrap(err, "error generating auth header")
		}
		return &kinds.RESTAuthBasic{
			Kind:     kinds.RESTAuthKindBasic,
			Username: this.Username,
			Password: this.Password,
			Headers: map[string]string{
				"Authorization": header,
			},
		}, nil
	default:
		return nil, errors.Errorf("unknown auth kind: %s", this.Kind)
	}
}
