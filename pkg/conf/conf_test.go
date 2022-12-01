package conf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/utils"
	libresources "github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func tempdir(t testing.TB) string {
	t.Helper()

	name, err := os.MkdirTemp("", "cli_test")
	if err != nil {
		t.Fatalf("tempdir: %s", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(name)
	})

	return name
}

func TestUserConfig(t *testing.T) {
	t.Run("read missing", func(t *testing.T) {
		var assert = require.New(t)
		var homedir = tempdir(t)
		var path = filepath.Join(homedir, ".airplane", "config")

		_, err := ReadUserConfig(path)

		assert.Error(err)
		assert.True(errors.Is(err, ErrMissing))
	})

	t.Run("write missing dir", func(t *testing.T) {
		var assert = require.New(t)
		var homedir = tempdir(t)
		var path = filepath.Join(homedir, ".airplane", "config")

		err := writeUserConfig(path, UserConfig{
			Tokens: map[string]string{"airplane.dev": "foo"},
		})
		assert.NoError(err)

		cfg, err := ReadUserConfig(path)
		assert.NoError(err)
		assert.Equal("foo", cfg.Tokens["airplane.dev"])
	})

	t.Run("overwrite", func(t *testing.T) {
		var assert = require.New(t)
		var homedir = tempdir(t)
		var path = filepath.Join(homedir, ".airplane", "config")

		{
			err := writeUserConfig(path, UserConfig{
				Tokens: map[string]string{"airplane.dev": "foo"},
			})
			assert.NoError(err)

			cfg, err := ReadUserConfig(path)
			assert.NoError(err)
			assert.Equal("foo", cfg.Tokens["airplane.dev"])
		}

		{
			err := writeUserConfig(path, UserConfig{
				Tokens: map[string]string{"airplane.dev": "baz"},
			})
			assert.NoError(err)

			cfg, err := ReadUserConfig(path)
			assert.NoError(err)
			assert.Equal("baz", cfg.Tokens["airplane.dev"])
		}
	})
}

func TestDevConfig(t *testing.T) {
	t.Run("read missing", func(t *testing.T) {
		var assert = require.New(t)
		var dir = tempdir(t)
		var path = filepath.Join(dir, "dev.yaml")

		_, err := readDevConfig(path)

		assert.Error(err)
		assert.True(errors.Is(err, ErrMissing))
	})

	t.Run("write and read", func(t *testing.T) {
		var assert = require.New(t)
		var dir = tempdir(t)
		var path = filepath.Join(dir, DefaultDevConfigFileName)

		configs := map[string]string{
			"CONFIG_VAR": "value",
		}
		postgres := map[string]interface{}{
			"kind":     "postgres",
			"slug":     "db",
			"username": "postgres",
			"password": "password",
			"port":     "5432",
			"ssl":      "disable",
			// no ID is written
		}
		configResources := []map[string]interface{}{
			postgres,
		}
		err := writeDevConfig(&DevConfig{
			RawConfigVars: configs,
			Path:          path,
			RawResources:  configResources,
		})
		assert.NoError(err)

		cfg, err := readDevConfig(path)
		assert.NoError(err)

		for _, r := range cfg.Resources {
			assert.Contains(r.Resource.GetID(), utils.DevResourcePrefix)
		}

		assert.Equal(configResources, cfg.RawResources)
		assert.Equal(map[string]env.ResourceWithEnv{
			"db": {
				Resource: &kinds.PostgresResource{
					BaseResource: libresources.BaseResource{
						Kind: kinds.ResourceKindPostgres,
						Slug: "db",
						ID:   cfg.Resources["db"].Resource.GetID(), // Do not compare ID
					},
					Username: "postgres",
					Password: "password",
					Port:     "5432",
					SSLMode:  "disable",
					DSN:      "postgres://postgres:password@:5432?sslmode=disable", // Calculated fields should be set
				},
				Remote: false,
			},
		}, cfg.Resources)

		assert.Equal(configs, cfg.RawConfigVars)
		// clear the ID so we can compare the rest of the config var
		for name, c := range cfg.ConfigVars {
			assert.Contains(c.ID, utils.DevConfigPrefix)
			c.ID = ""
			cfg.ConfigVars[name] = c
		}
		assert.Equal(map[string]env.ConfigWithEnv{
			"CONFIG_VAR": {
				Config: api.Config{
					Name:     "CONFIG_VAR",
					Value:    "value",
					IsSecret: false,
				},
				Remote: false,
				Env:    env.NewLocalEnv(),
			},
		}, cfg.ConfigVars)
	})
}
