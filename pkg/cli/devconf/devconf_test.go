package devconf

import (
	"path/filepath"
	"testing"

	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/airplanedev/cli/pkg/cli/dev/env"
	libresources "github.com/airplanedev/cli/pkg/cli/resources"
	"github.com/airplanedev/cli/pkg/cli/resources/kinds"
	"github.com/airplanedev/cli/pkg/testutils"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestDevConfigMissing(t *testing.T) {
	require := require.New(t)
	var dir = testutils.Tempdir(t)
	var path = filepath.Join(dir, "dev.yaml")

	_, err := readDevConfig(path)

	require.Error(err)
	require.True(errors.Is(err, ErrMissing))
}

func TestDevConfigResources(t *testing.T) {
	require := require.New(t)
	var dir = testutils.Tempdir(t)
	var path = filepath.Join(dir, DefaultDevConfigFileName)

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
		Path:         path,
		RawResources: configResources,
	})
	require.NoError(err)

	cfg, err := readDevConfig(path)
	require.NoError(err)

	for _, r := range cfg.Resources {
		require.Contains(r.Resource.GetID(), utils.DevResourcePrefix)
	}

	require.Equal(configResources, cfg.RawResources)
	require.Equal(map[string]env.ResourceWithEnv{
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
				DSN:      "postgres://postgres:password@:5432?application_name=Airplane&sslmode=disable", // Calculated fields should be set
			},
			Remote: false,
		},
	}, cfg.Resources)
}

func TestDevConfigConfigVars(t *testing.T) {
	require := require.New(t)
	var dir = testutils.Tempdir(t)
	var path = filepath.Join(dir, DefaultDevConfigFileName)

	configs := map[string]string{
		"CONFIG_VAR": "value",
	}
	err := writeDevConfig(&DevConfig{
		RawConfigVars: configs,
		Path:          path,
	})
	require.NoError(err)

	cfg, err := readDevConfig(path)
	require.NoError(err)

	require.Equal(configs, cfg.RawConfigVars)
	// clear the ID so we can compare the rest of the config var
	for name, c := range cfg.ConfigVars {
		require.Contains(c.ID, utils.DevConfigPrefix)
		c.ID = ""
		cfg.ConfigVars[name] = c
	}
	require.Equal(map[string]env.ConfigWithEnv{
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
}

func TestDevConfigEnvVars(t *testing.T) {
	require := require.New(t)
	var dir = testutils.Tempdir(t)
	var path = filepath.Join(dir, DefaultDevConfigFileName)

	err := writeDevConfig(&DevConfig{
		EnvVars: map[string]string{
			"ENV_VAR_1": "value_1",
			"ENV_VAR_2": "value_2",
		},
		Path: path,
	})
	require.NoError(err)

	cfg, err := readDevConfig(path)
	require.NoError(err)

	require.Equal(map[string]string{
		"ENV_VAR_1": "value_1",
		"ENV_VAR_2": "value_2",
	}, cfg.EnvVars)

	err = cfg.SetEnvVar("ENV_VAR_3", "value_3")
	require.NoError(err)
	require.Equal(map[string]string{
		"ENV_VAR_1": "value_1",
		"ENV_VAR_2": "value_2",
		"ENV_VAR_3": "value_3",
	}, cfg.EnvVars)

	err = cfg.DeleteEnvVar("ENV_VAR_1")
	require.NoError(err)
	require.Equal(map[string]string{
		"ENV_VAR_2": "value_2",
		"ENV_VAR_3": "value_3",
	}, cfg.EnvVars)
}
