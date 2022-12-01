package conf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	libresources "github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var DefaultDevConfigFileName = "airplane.dev.yaml"

// DevConfig represents an airplane dev configuration.
type DevConfig struct {
	// RawResources is a list of resources that represents what the user sees in the dev config file.
	RawResources []map[string]interface{} `json:"resources" yaml:"resources"`
	// Configs is a map of config variables in the format that the user sees in the dev config file.
	RawConfigVars map[string]string `json:"configVars" yaml:"configVars"`

	// Resources is a mapping from slug to external resource.
	Resources  map[string]env.ResourceWithEnv `json:"-" yaml:"-"`
	ConfigVars map[string]env.ConfigWithEnv   `json:"-" yaml:"-"`

	// Path is the location that the dev config file was loaded from and where updates will be written to.
	Path string `json:"-" yaml:"-"`

	mu sync.Mutex
}

// NewDevConfig returns a default dev config.
func NewDevConfig(path string) *DevConfig {
	return &DevConfig{
		RawConfigVars: map[string]string{},
		RawResources:  []map[string]interface{}{},
		ConfigVars:    map[string]env.ConfigWithEnv{},
		Resources:     map[string]env.ResourceWithEnv{},
		Path:          path,
	}
}

// updateRawResources needs to be called whenever Resources is mutated, to keep RawResources in sync
// the caller of updateRawResources should have the lock on the DevConfig
func (d *DevConfig) updateRawResources() error {
	resourceList := make([]libresources.Resource, 0, len(d.RawResources))

	for _, r := range d.Resources {
		resourceList = append(resourceList, r.Resource)
	}

	// TODO: Use yaml.Marshal/Unmarshal once we've added yaml struct tags to external resource structs.
	buf, err := json.Marshal(resourceList)
	if err != nil {
		return errors.Wrap(err, "marshaling resources")
	}

	d.RawResources = []map[string]interface{}{}
	if err := json.Unmarshal(buf, &d.RawResources); err != nil {
		return errors.Wrap(err, "unmarshalling into raw resources")
	}
	for _, resource := range d.RawResources {
		// hide the resource ID so it doesn't get marshaled into the dev config YAML
		delete(resource, "id")
	}

	return nil
}

// SetResource updates a resource in the dev config file, creating it if necessary.
func (d *DevConfig) SetResource(slug string, r libresources.Resource) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.Resources[slug] = env.ResourceWithEnv{
		Resource: r,
		Remote:   false,
	}

	if err := d.updateRawResources(); err != nil {
		return errors.Wrap(err, "updating raw resources")
	}

	if err := writeDevConfig(d); err != nil {
		return errors.Wrap(err, "writing dev config")
	}
	logger.Log("Wrote resource %s to dev config file at %s", slug, d.Path)

	return nil
}

// RemoveResource removes the resource in the dev config file with the given slug, if it exists.
func (d *DevConfig) RemoveResource(slug string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for s := range d.Resources {
		if s == slug {
			delete(d.Resources, s)
		}
	}

	if err := d.updateRawResources(); err != nil {
		return errors.Wrap(err, "updating raw resources")
	}

	if err := writeDevConfig(d); err != nil {
		return errors.Wrap(err, "writing dev config")
	}

	return nil
}

// updateRawConfigs needs to be called whenever ConfigVars is mutated to keep RawConfigVars in sync
// the caller of updateRawConfigs should have the lock on the DevConfig
func (d *DevConfig) updateRawConfigs() error {
	configMap := make(map[string]string, len(d.ConfigVars))
	for _, c := range d.ConfigVars {
		configMap[c.Name] = c.Value
	}

	d.RawConfigVars = configMap
	return nil
}

// SetConfigVar updates a config in the dev config file, creating it if necessary.
func (d *DevConfig) SetConfigVar(key string, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if cfg, ok := d.ConfigVars[key]; ok {
		cfg.Value = value
		d.ConfigVars[key] = cfg
	} else {
		d.ConfigVars[key] = env.ConfigWithEnv{
			Config: api.Config{
				ID:       utils.GenerateDevConfigID(key),
				Name:     key,
				Value:    value,
				IsSecret: false,
			},
			Remote: false,
			Env:    env.NewLocalEnv(),
		}
	}

	if err := d.updateRawConfigs(); err != nil {
		return errors.Wrap(err, "updating raw configs")
	}

	if err := writeDevConfig(d); err != nil {
		return err
	}

	logger.Log("Successfully wrote config variable %q to dev config file.", key)
	return nil
}

// RemoveConfigVar deletes the config from the dev config file, if it exists.
func (d *DevConfig) RemoveConfigVar(key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.ConfigVars[key]; !ok {
		return errors.Errorf("Config variable %q not found in dev config file", key)
	}

	delete(d.ConfigVars, key)

	if err := d.updateRawConfigs(); err != nil {
		return errors.Wrap(err, "updating raw configs")
	}

	if err := writeDevConfig(d); err != nil {
		return err
	}

	logger.Log("Deleted config variable %q from dev config file.", key)
	return nil
}

// LoadConfigFile reads the contents of the dev config file at the path
func (d *DevConfig) LoadConfigFile() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	config, err := NewDevConfigFile(d.Path)
	if err != nil {
		return err
	}
	d.RawConfigVars = config.RawConfigVars
	d.ConfigVars = config.ConfigVars
	d.RawResources = config.RawResources
	d.Resources = config.Resources
	return nil
}

func readDevConfig(path string) (*DevConfig, error) {
	cfg := &DevConfig{}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrMissing
	}

	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read config")
	}

	if err := yaml.Unmarshal(buf, cfg); err != nil {
		return nil, errors.Wrap(err, "unmarshal config")
	}

	// Load in resources
	slugToResource := map[string]env.ResourceWithEnv{}
	for _, r := range cfg.RawResources {
		kind, ok := r["kind"]
		if !ok {
			return nil, errors.New("missing kind property in resource")
		}

		kindStr, ok := kind.(string)
		if !ok {
			return nil, errors.Errorf("expected kind type to be string, got %T", kind)
		}

		slug, ok := r["slug"]
		if !ok {
			return nil, errors.New("missing slug property in resource")
		}

		slugStr, ok := r["slug"].(string)
		if !ok {
			return nil, errors.Errorf("expected slug type to be string, got %T", slug)
		}

		res, err := libresources.GetResource(libresources.ResourceKind(kindStr), r)
		if err != nil {
			return nil, errors.Wrap(err, "getting resource from raw resource")
		}

		// generate the resource ID so the dev config file doesn't need to have it
		if err := res.UpdateBaseResource(libresources.BaseResource{
			ID: utils.GenerateDevResourceID(slugStr),
		}); err != nil {
			return nil, errors.Wrap(err, "updating base resource")
		}

		slugToResource[slugStr] = env.ResourceWithEnv{
			Resource: res,
			Remote:   false,
		}
	}
	cfg.Resources = slugToResource

	// Load in configs
	nameToConfig := make(map[string]env.ConfigWithEnv, len(cfg.RawConfigVars))
	for name, value := range cfg.RawConfigVars {
		nameToConfig[name] = env.ConfigWithEnv{
			Config: api.Config{
				ID:       utils.GenerateDevConfigID(name),
				Name:     name,
				Value:    value,
				IsSecret: false,
			},
			Remote: false,
			Env:    env.NewLocalEnv(),
		}
	}
	cfg.ConfigVars = nameToConfig

	cfg.Path = path

	return cfg, nil
}

func writeDevConfig(config *DevConfig) error {
	if err := os.MkdirAll(filepath.Dir(config.Path), 0777); err != nil {
		return errors.Wrap(err, "mkdir")
	}
	buf, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if _, err := os.Stat(config.Path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f, createErr := os.Create(config.Path)
			if createErr != nil {
				return errors.Wrap(createErr, "creating dev config file")
			}
			f.Close()
			logger.Log("Created dev config file at %s", config.Path)
		} else {
			return errors.Wrap(err, "checking if dev config file exists")
		}
	}

	if err := os.WriteFile(config.Path, buf, 0644); err != nil {
		return errors.Wrap(err, "write config")
	}

	return nil
}

// NewDevConfigFile attempts to load in the dev config file at the provided path
// and returns a new DevConfig
func NewDevConfigFile(devConfigPath string) (*DevConfig, error) {
	var devConfig *DevConfig
	var devConfigLoaded bool
	if devConfigPath != "" {
		var err error
		devConfig, err = readDevConfig(devConfigPath)
		if err != nil {
			if !errors.Is(err, ErrMissing) {
				return nil, errors.Wrap(err, "unable to read dev config")
			}
		} else {
			devConfigLoaded = true
		}
	}

	if devConfigLoaded {
		logger.Log("%v Loaded dev config from %s", logger.Yellow(time.Now().Format(logger.TimeFormatNoDate)), devConfigPath)
	} else {
		logger.Debug("Using empty dev config")
		devConfig = NewDevConfig(devConfigPath)
	}

	return devConfig, nil
}
