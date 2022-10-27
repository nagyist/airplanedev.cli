package conf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/lib/pkg/resources"
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
		var path = filepath.Join(dir, "dev.yaml")

		configs := map[string]string{
			"CONFIG_VAR": "value",
		}
		postgres := map[string]interface{}{
			"kind":     "postgres",
			"slug":     "db",
			"username": "postgres",
			"password": "password",
			// no ID is written
		}
		configResources := []map[string]interface{}{
			postgres,
		}
		err := writeDevConfig(&DevConfig{
			ConfigVars:   configs,
			Path:         path,
			RawResources: configResources,
		})
		assert.NoError(err)

		cfg, err := readDevConfig(path)
		assert.NoError(err)
		assert.Equal(configs, cfg.ConfigVars)
		// reading from the dev config should generate the ID into RawResources
		postgres["id"] = "res-db"
		assert.Equal(configResources, cfg.RawResources)
		assert.Equal(map[string]env.ResourceWithEnv{
			"db": {
				Resource: &kinds.PostgresResource{
					BaseResource: resources.BaseResource{
						Kind: kinds.ResourceKindPostgres,
						Slug: "db",
						ID:   "res-db",
					},
					Username: "postgres",
					Password: "password",
				},
				Remote: false,
			},
		}, cfg.Resources)
	})
}
