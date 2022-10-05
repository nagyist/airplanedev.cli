package conf

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// UserConfig represents user-specific configuration for the CLI.
type UserConfig struct {
	Tokens          map[string]string `json:"tokens,omitempty"`
	EnableTelemetry *bool             `json:"enableTelemetry,omitempty"`
}

// Path returns the default config defaultUserConfigPath.
func defaultUserConfigPath() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		// TODO(amir): friendly output.
		panic("$HOME environment variable must be set")
	}
	return filepath.Join(
		homedir,
		".airplane",
		"config",
	)
}

// ReadDefaultUserConfig reads the configuration from the default location.
func ReadDefaultUserConfig() (UserConfig, error) {
	return ReadUserConfig(defaultUserConfigPath())
}

// ReadUserConfig reads the configuration from `defaultUserConfigPath`.
func ReadUserConfig(path string) (UserConfig, error) {
	var cfg UserConfig

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, ErrMissing
	}

	buf, err := os.ReadFile(path)
	if err != nil {
		return cfg, errors.Wrap(err, "read config")
	}

	if err := json.Unmarshal(buf, &cfg); err != nil {
		return cfg, errors.Wrap(err, "unmarshal config")
	}

	return cfg, nil
}

// WriteUserConfig writes the configuration to the given defaultUserConfigPath.
func WriteUserConfig(path string, cfg UserConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return errors.Wrap(err, "mkdir")
	}

	buf, err := json.MarshalIndent(cfg, "", "	")
	if err != nil {
		return errors.Wrap(err, "marshal config")
	}

	if err := os.WriteFile(path, buf, 0600); err != nil {
		return errors.Wrap(err, "write config")
	}

	return nil
}

// WriteDefaultUserConfig saves configuration to the default location.
func WriteDefaultUserConfig(cfg UserConfig) error {
	return WriteUserConfig(defaultUserConfigPath(), cfg)
}
