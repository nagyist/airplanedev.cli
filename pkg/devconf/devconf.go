package devconf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/dev/env"
	libresources "github.com/airplanedev/cli/pkg/resources"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var (
	// ErrMissing is returned when the config file does not exist.
	ErrMissing = errors.New("conf: config file does not exist")
)

var DefaultDevConfigFileName = "airplane.dev.yaml"

// DevConfig represents an airplane dev configuration.
type DevConfig struct {
	// RawResources is a list of resources that represents what the user sees in the dev config file.
	RawResources []map[string]interface{} `json:"resources" yaml:"resources"`
	// Configs is a map of config variables in the format that the user sees in the dev config file.
	RawConfigVars map[string]string `json:"configVars" yaml:"configVars"`
	EnvVars       map[string]string `json:"envVars" yaml:"envVars"`

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
		RawResources:  []map[string]interface{}{},
		Resources:     map[string]env.ResourceWithEnv{},
		RawConfigVars: map[string]string{},
		ConfigVars:    map[string]env.ConfigWithEnv{},
		EnvVars:       map[string]string{},
		Path:          path,
	}
}

// Update reads the contents of the dev config file.
func (d *DevConfig) Update() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	config, err := LoadDevConfigFile(d.Path)
	if err != nil {
		return err
	}

	// We want to avoid concurrent updates to the dev config file, so we maintain a mutex in the dev config struct,
	// and copy the contents of the dev config file into the struct whenever we update it.
	d.RawConfigVars = config.RawConfigVars
	d.ConfigVars = config.ConfigVars
	d.RawResources = config.RawResources
	d.Resources = config.Resources
	d.EnvVars = config.EnvVars
	return nil
}

// updateRawResources needs to be called whenever Resources is mutated, to keep RawResources in sync
// the caller of updateRawResources should have the lock on the DevConfig
func (d *DevConfig) updateRawResources() error {
	resourceList := make([]libresources.Resource, 0, len(d.RawResources))

	for _, r := range d.Resources {
		// Don't save calculated fields (e.g. DSN) to the dev config file.
		r.Resource.ScrubCalculatedFields()
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

	if err := writeDevConfig(d); err != nil {
		return errors.Wrap(err, "writing dev config")
	}

	// Recompute calculated fields, since we need them for any tasks that use resources.
	for _, resource := range d.Resources {
		if err := resource.Resource.Calculate(); err != nil {
			return errors.Wrap(err, "computing calculated resource fields")
		}
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

	logger.Log("Wrote resource %s to dev config file at %s", slug, d.Path)

	return nil
}

// DeleteResource removes the resource in the dev config file with the given slug, if it exists.
func (d *DevConfig) DeleteResource(slug string) error {
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

	return nil
}

// updateRawConfigs needs to be called whenever ConfigVars is mutated to keep RawConfigVars in sync.
// The caller of updateRawConfigs should have the lock on the DevConfig
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

// DeleteConfigVar deletes the config from the dev config file, if it exists.
func (d *DevConfig) DeleteConfigVar(key string) error {
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

// SetEnvVar updates an env var in the dev config file, creating it if necessary.
func (d *DevConfig) SetEnvVar(key string, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.EnvVars[key] = value

	if err := writeDevConfig(d); err != nil {
		return err
	}

	logger.Log("Successfully wrote environment variable %q to dev config file.", key)
	return nil
}

// DeleteEnvVar deletes an env var from the dev config file, if it exists.
func (d *DevConfig) DeleteEnvVar(key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.EnvVars[key]; !ok {
		return errors.Errorf("Environment variable %q not found in dev config file", key)
	}

	delete(d.EnvVars, key)

	if err := writeDevConfig(d); err != nil {
		return err
	}

	logger.Log("Deleted environment variable %q from dev config file.", key)
	return nil
}

// readDevConfig reads in the dev config file at the given path and converts the raw config fields into structs that
// are used during local development.
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

		// Compute calculated resource fields since we scrub them before saving the resource to the dev config file.
		if err := res.Calculate(); err != nil {
			return nil, errors.Wrap(err, "computing calculated resource fields")
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

	if cfg.EnvVars == nil {
		cfg.EnvVars = map[string]string{}
	}

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

// LoadDevConfigFile attempts to load in the dev config file at the provided path and returns a new DevConfig struct.
// If no such dev config file exists at the given path, an empty dev config struct is returned with the path set to the
// provided path such that on the next save, the dev config file will be created at that path.
func LoadDevConfigFile(devConfigPath string) (*DevConfig, error) {
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
