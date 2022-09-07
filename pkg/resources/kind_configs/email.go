package kind_configs

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

const (
	KindMailgun  resources.ResourceKind = "mailgun"
	KindSendGrid resources.ResourceKind = "sendgrid"
	KindSMTP     resources.ResourceKind = "smtp"
)

func init() {
	ResourceKindToKindConfig[KindMailgun] = &MailgunKindConfig{}
	ResourceKindToKindConfig[KindSendGrid] = &SendGridKindConfig{}
	ResourceKindToKindConfig[KindSMTP] = &SMTPKindConfig{}
}

type MailgunKindConfig struct {
	APIKey string `json:"apiKey" yaml:"apiKey"`
	Domain string `json:"domain" yaml:"domain"`
}

var _ ResourceConfigValues = &MailgunKindConfig{}

func (kc *MailgunKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*MailgunKindConfig)
	if !ok {
		return errors.Errorf("expected *MailgunKindConfig got %T", cv)
	}
	if c.APIKey != "" {
		kc.APIKey = c.APIKey
	}
	kc.Domain = c.Domain
	return nil
}

func (kc MailgunKindConfig) Validate() error {
	r, err := kc.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func (kc MailgunKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	return &kinds.MailgunResource{
		BaseResource: base,
		APIKey:       kc.APIKey,
		Domain:       kc.Domain,
	}, nil
}

type SendGridKindConfig struct {
	APIKey string `json:"apiKey" yaml:"apiKey"`
}

var _ ResourceConfigValues = &SendGridKindConfig{}

func (kc *SendGridKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*SendGridKindConfig)
	if !ok {
		return errors.Errorf("expected *SendGridKindConfig got %T", cv)
	}
	if c.APIKey != "" {
		kc.APIKey = c.APIKey
	}
	return nil
}

func (kc SendGridKindConfig) Validate() error {
	r, err := kc.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func (kc SendGridKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	return &kinds.SendGridResource{
		BaseResource: base,
		APIKey:       kc.APIKey,
	}, nil
}

type SMTPKindConfig struct {
	AuthConfig SMTPAuthConfig `json:"authConfig" yaml:"authConfig"`
	Hostname   string         `json:"hostname" yaml:"hostname"`
	Port       string         `json:"port" yaml:"port"`
}

var _ ResourceConfigValues = &SMTPKindConfig{}

func (kc *SMTPKindConfig) Update(cv ResourceConfigValues) error {
	c, ok := cv.(*SMTPKindConfig)
	if !ok {
		return errors.Errorf("expected *SMTPKindConfig got %T", cv)
	}
	kc.AuthConfig.update(c.AuthConfig)
	kc.Hostname = c.Hostname
	kc.Port = c.Port
	return nil
}

func (kc SMTPKindConfig) Validate() error {
	r, err := kc.ToExternalResource(resources.BaseResource{})
	if err != nil {
		return err
	}
	return r.Validate()
}

func (kc SMTPKindConfig) ToExternalResource(base resources.BaseResource) (resources.Resource, error) {
	authConfig, err := kc.AuthConfig.toExternalSMTPAuth()
	if err != nil {
		return nil, err
	}
	return &kinds.SMTPResource{
		BaseResource: base,
		Auth:         authConfig,
		Hostname:     kc.Hostname,
		Port:         kc.Port,
	}, nil
}

type SMTPAuthConfig struct {
	Kind    SMTPAuthConfigKind     `json:"kind" yaml:"kind"`
	Plain   *SMTPAuthConfigPlain   `json:"plain,omitempty" yaml:"plain,omitempty"`
	CRAMMD5 *SMTPAuthConfigCRAMMD5 `json:"crammd5,omitempty" yaml:"crammd5,omitempty"`
	Login   *SMTPAuthConfigLogin   `json:"login,omitempty" yaml:"login,omitempty"`
}

type SMTPAuthConfigKind string

const (
	KindPlain   SMTPAuthConfigKind = "plain"
	KindCRAMMD5 SMTPAuthConfigKind = "crammd5"
	KindLogin   SMTPAuthConfigKind = "login"
)

type SMTPAuthConfigPlain struct {
	Identity string `json:"identity"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type SMTPAuthConfigCRAMMD5 struct {
	Username string `json:"username"`
	Secret   string `json:"secret"`
}

type SMTPAuthConfigLogin struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (ac *SMTPAuthConfig) update(c SMTPAuthConfig) {
	if ac.Plain == nil && c.Plain != nil {
		ac.reset()
		ac.Plain = &SMTPAuthConfigPlain{}
	} else if ac.CRAMMD5 == nil && c.CRAMMD5 != nil {
		ac.reset()
		ac.CRAMMD5 = &SMTPAuthConfigCRAMMD5{}
	} else if ac.Login == nil && c.Login != nil {
		ac.reset()
		ac.Login = &SMTPAuthConfigLogin{}
	}

	if c.Plain != nil {
		ac.Plain.Identity = c.Plain.Identity
		ac.Plain.Username = c.Plain.Username
		// Don't clobber over password if empty
		if c.Plain.Password != "" {
			ac.Plain.Password = c.Plain.Password
		}
	} else if c.CRAMMD5 != nil {
		ac.CRAMMD5.Username = c.CRAMMD5.Username
		if c.CRAMMD5.Secret != "" {
			ac.CRAMMD5.Secret = c.CRAMMD5.Secret
		}
	} else if c.Login != nil {
		ac.Login.Username = c.Login.Username
		// Don't clobber over password if empty
		if c.Login.Password != "" {
			ac.Login.Password = c.Login.Password
		}
	}
}

func (ac *SMTPAuthConfig) reset() {
	ac.Plain = nil
	ac.CRAMMD5 = nil
	ac.Login = nil
}

func (ac SMTPAuthConfig) validate() error { // nolint:unused
	if ac.Plain != nil {
		// Identity can be & usually is empty string, no need to check.
		if ac.Plain.Username == "" {
			return errors.New("missing username for plain SMTP auth")
		}
		if ac.Plain.Password == "" {
			return errors.New("missing password for plain SMTP auth")
		}
	} else if ac.CRAMMD5 != nil {
		if ac.CRAMMD5.Username == "" {
			return errors.New("missing username for CRAMMD5 SMTP auth")
		}
		if ac.CRAMMD5.Secret == "" {
			return errors.New("missing secret for CRAMMD5 SMTP auth")
		}
	} else if ac.Login != nil {
		if ac.Login.Username == "" {
			return errors.New("missing username for login SMTP auth")
		}
		if ac.Login.Password == "" {
			return errors.New("missing password for login SMTP auth")
		}
	} else {
		return errors.New("missing SMTP auth")
	}
	return nil
}

func (ac SMTPAuthConfig) toExternalSMTPAuth() (kinds.SMTPAuth, error) {
	if ac.Plain != nil {
		return &kinds.SMTPAuthPlain{
			Kind:     kinds.EmailSMTPAuthKindPlain,
			Identity: ac.Plain.Identity,
			Username: ac.Plain.Username,
			Password: ac.Plain.Password,
		}, nil
	} else if ac.CRAMMD5 != nil {
		return &kinds.SMTPAuthCRAMMD5{
			Kind:     kinds.EmailSMTPAuthKindCRAMMD5,
			Username: ac.CRAMMD5.Username,
			Secret:   ac.CRAMMD5.Secret,
		}, nil
	} else if ac.Login != nil {
		return &kinds.SMTPAuthLogin{
			Kind:     kinds.EmailSMTPAuthKindLogin,
			Username: ac.Login.Username,
			Password: ac.Login.Password,
		}, nil
	} else {
		return nil, errors.New("missing SMTP auth")
	}
}
