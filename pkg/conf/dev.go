package conf

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/resource"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// DevConfig represents an airplane dev configuration.
type DevConfig struct {
	Env              map[string]string                 `yaml:"env"`
	Resources        map[string]map[string]interface{} `yaml:"resources"`
	DecodedResources map[string]resource.Resource
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

	slugToResource := map[string]resource.Resource{}
	for slug, r := range cfg.Resources {
		if kind, ok := r["kind"]; ok {
			if kindStr, ok := kind.(string); ok {
				slugToResource[slug], err = resource.GetResource(resource.ResourceKind(kindStr), r)
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
