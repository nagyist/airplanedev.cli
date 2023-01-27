package dev

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/env"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/stretchr/testify/require"
)

func TestSetEnvVars(t *testing.T) {
	require := require.New(t)

	tempDir := t.TempDir()
	entrypoint := filepath.Join(tempDir, "my_task.airplane.ts")

	envVarValue := "foo"
	configVarValue := "bar"
	configName := "my_config"

	runConfig := LocalRunConfig{
		LocalClient:  &api.Client{},
		RemoteClient: &api.MockClient{},
		Kind:         build.TaskKindNode,
		TaskEnvVars: map[string]libapi.EnvVarValue{
			"ENV_VAR_FROM_VALUE": {
				Value: &envVarValue,
			},
			"ENV_VAR_FROM_CONFIG": {
				Config: &configName,
			},
		},
		ConfigVars: map[string]env.ConfigWithEnv{
			configName: {
				Config: api.Config{
					Name:  configName,
					Value: configVarValue,
				},
			},
		},
	}

	r, err := runtime.Lookup(entrypoint, runConfig.Kind)
	require.NoError(err)

	envVars, err := getEnvVars(context.Background(), runConfig, r, entrypoint, libapi.EvaluateTemplateRequest{})
	require.NoError(err)

	require.True(requireEnvVarExists(envVars, "ENV_VAR_FROM_VALUE", envVarValue))
	require.True(requireEnvVarExists(envVars, "ENV_VAR_FROM_CONFIG", configVarValue))
}

func TestSetEnvVarsOverride(t *testing.T) {
	require := require.New(t)

	tempDir := t.TempDir()
	entrypoint := filepath.Join(tempDir, "my_task.airplane.ts")

	envVarValue := "foo"
	envVarValueFromDevConfig := "baz"

	runConfig := LocalRunConfig{
		LocalClient:  &api.Client{},
		RemoteClient: &api.MockClient{},
		Kind:         build.TaskKindNode,
		TaskEnvVars: map[string]libapi.EnvVarValue{
			"ENV_VAR_FROM_VALUE": {
				Value: &envVarValue,
			},
		},
		EnvVars: map[string]string{
			"ENV_VAR_FROM_VALUE": envVarValueFromDevConfig,
		},
	}

	r, err := runtime.Lookup(entrypoint, runConfig.Kind)
	require.NoError(err)

	envVars, err := getEnvVars(context.Background(), runConfig, r, entrypoint, libapi.EvaluateTemplateRequest{})
	require.NoError(err)

	require.True(requireEnvVarExists(envVars, "ENV_VAR_FROM_VALUE", envVarValueFromDevConfig))
}

func requireEnvVarExists(envVars []string, key, value string) bool {
	for _, pair := range envVars {
		parts := strings.SplitN(pair, "=", 2)
		if key == parts[0] && value == parts[1] {
			return true
		}
	}

	return false
}
