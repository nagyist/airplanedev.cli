package conf

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/resources"
	_ "github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var DefaultDevConfigFileName = "airplane.dev.yaml"

// DevConfig represents an airplane dev configuration.
type DevConfig struct {
	Env              map[string]string                 `yaml:"env"`
	Resources        map[string]map[string]interface{} `yaml:"resources"`
	DecodedResources map[string]resources.Resource     `yaml:"-"`
}

func ReadDevConfig(path string) (DevConfig, error) {
	var cfg DevConfig

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DevConfig{}, ErrMissing
	}

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return DevConfig{}, errors.Wrap(err, "read config")
	}

	if err := yaml.Unmarshal(buf, &cfg); err != nil {
		return DevConfig{}, errors.Wrap(err, "unmarshal config")
	}

	slugToResource := map[string]resources.Resource{}
	for slug, r := range cfg.Resources {
		if kind, ok := r["kind"]; ok {
			if kindStr, ok := kind.(string); ok {
				slugToResource[slug], err = resources.GetResource(resources.ResourceKind(kindStr), r)
				if err != nil {
					return DevConfig{}, errors.Wrap(err, "getting resource struct from kind")
				}
			} else {
				return DevConfig{}, errors.Errorf("expected kind type string, got %T", r["kind"])
			}
		} else {
			return DevConfig{}, errors.New("missing kind property in resource")
		}
	}

	cfg.DecodedResources = slugToResource

	return cfg, nil
}

func WriteDevConfig(path string, config DevConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return errors.Wrap(err, "mkdir")
	}

	buf, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(path, buf, 0600); err != nil {
		return errors.Wrap(err, "write config")
	}

	return nil
}

func PromptDevConfigFileCreation(defaultPath string) (string, error) {
	ok, err := utils.Confirm("Dev config file not found - would you like to create one?")
	if err != nil {
		return "", err
	} else if !ok {
		return "", nil
	}

	var path string
	if err := survey.AskOne(
		&survey.Input{
			Message: "Where would you like to create the dev config file?",
			Default: defaultPath,
		},
		&path,
	); err != nil {
		return "", err
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return path, nil
}
