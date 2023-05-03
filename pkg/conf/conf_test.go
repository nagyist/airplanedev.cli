package conf

import (
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/testutils"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestUserConfig(t *testing.T) {
	t.Run("read missing", func(t *testing.T) {
		var assert = require.New(t)
		var homedir = testutils.Tempdir(t)
		var path = filepath.Join(homedir, ".airplane", "config")

		_, err := ReadUserConfig(path)

		assert.Error(err)
		assert.True(errors.Is(err, ErrMissing))
	})

	t.Run("write missing dir", func(t *testing.T) {
		var assert = require.New(t)
		var homedir = testutils.Tempdir(t)
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
		var homedir = testutils.Tempdir(t)
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
