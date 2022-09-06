package conf

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func tempdir(t testing.TB) string {
	t.Helper()

	name, err := ioutil.TempDir("", "cli_test")
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

		err := WriteUserConfig(path, UserConfig{
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
			err := WriteUserConfig(path, UserConfig{
				Tokens: map[string]string{"airplane.dev": "foo"},
			})
			assert.NoError(err)

			cfg, err := ReadUserConfig(path)
			assert.NoError(err)
			assert.Equal("foo", cfg.Tokens["airplane.dev"])
		}

		{
			err := WriteUserConfig(path, UserConfig{
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

		_, err := ReadDevConfig(path)

		assert.Error(err)
		assert.True(errors.Is(err, ErrMissing))
	})

	t.Run("write and read", func(t *testing.T) {
		var assert = require.New(t)
		var dir = tempdir(t)
		var path = filepath.Join(dir, "dev.yaml")

		env := map[string]string{
			"ENV_VAR": "value",
		}
		configResources := []map[string]interface{}{
			{
				"kind":     "postgres",
				"slug":     "db",
				"username": "postgres",
				"password": "password",
			},
		}
		err := WriteDevConfig(&DevConfig{
			Path:         path,
			EnvVars:      env,
			RawResources: configResources,
		})
		assert.NoError(err)

		cfg, err := ReadDevConfig(path)
		assert.NoError(err)
		assert.Equal(env, cfg.EnvVars)
		assert.Equal(configResources, cfg.RawResources)
		assert.Equal(map[string]resources.Resource{
			"db": kinds.PostgresResource{
				BaseResource: resources.BaseResource{
					Kind: kinds.ResourceKindPostgres,
					Slug: "db",
				},
				Username: "postgres",
				Password: "password",
			},
		}, cfg.Resources)
	})
}
