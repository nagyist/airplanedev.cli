package dev

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/stretchr/testify/assert"
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

func TestGetEnvVarsForView(t *testing.T) {
	testCases := []struct {
		desc            string
		config          GetEnvVarsForViewConfig
		expectedEnvVars map[string]string
	}{
		{
			desc: "Gets env vars from value from view",
			config: GetEnvVarsForViewConfig{
				ViewEnvVars: map[string]libapi.EnvVarValue{
					"ENV_VAR_FROM_VALUE": {
						Value: pointers.String("foo"),
					},
				},
			},
			expectedEnvVars: map[string]string{
				"ENV_VAR_FROM_VALUE": "foo",
			},
		},
		{
			desc: "Gets env vars from config from dev config file",
			config: GetEnvVarsForViewConfig{
				ViewEnvVars: map[string]libapi.EnvVarValue{
					"ENV_VAR_FROM_CONFIG": {
						Config: pointers.String("my_config"),
					},
				},
				ConfigVars: map[string]env.ConfigWithEnv{
					"my_config": {
						Config: api.Config{
							Name:  "my_config",
							Value: "foo",
						},
					},
				},
			},
			expectedEnvVars: map[string]string{
				"ENV_VAR_FROM_CONFIG": "foo",
			},
		},
		{
			desc: "Gets env vars from value from dev config file",
			config: GetEnvVarsForViewConfig{
				DevConfigEnvVars: map[string]string{
					"ENV_VAR_FROM_VALUE": "foo",
				},
			},
			expectedEnvVars: map[string]string{
				"ENV_VAR_FROM_VALUE": "foo",
			},
		},
		{
			desc: "Env var in dev config file same env var in view",
			config: GetEnvVarsForViewConfig{
				DevConfigEnvVars: map[string]string{
					"ENV_VAR_FROM_VALUE": "bar",
				},
				ViewEnvVars: map[string]libapi.EnvVarValue{
					"ENV_VAR_FROM_VALUE": {
						Value: pointers.String("foo"),
					},
				},
			},
			expectedEnvVars: map[string]string{
				"ENV_VAR_FROM_VALUE": "bar",
			},
		},
		{
			desc: "Adds built in env vars",
			config: GetEnvVarsForViewConfig{
				Slug:    "my_slug",
				Name:    "my_name",
				ViewURL: "https://app.airplane.so/my_slug/my_name",
				AuthInfo: api.AuthInfoResponse{
					Team: &api.TeamInfo{
						ID: "my_team_id",
					},
					User: &api.UserInfo{
						ID:    "my_user_id",
						Name:  "my_user_name",
						Email: "my_user_email",
					},
				},
			},
			expectedEnvVars: map[string]string{
				"AIRPLANE_ENV_ID":     "studio",
				"AIRPLANE_VIEW_SLUG":  "my_slug",
				"AIRPLANE_VIEW_NAME":  "my_name",
				"AIRPLANE_VIEW_URL":   "https://app.airplane.so/my_slug/my_name",
				"AIRPLANE_TEAM_ID":    "my_team_id",
				"AIRPLANE_USER_ID":    "my_user_id",
				"AIRPLANE_USER_NAME":  "my_user_name",
				"AIRPLANE_USER_EMAIL": "my_user_email",
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			envVars, err := GetEnvVarsForView(context.Background(), &api.MockClient{}, tC.config)
			require.NoError(err)

			for key, value := range tC.expectedEnvVars {
				assert.Contains(envVars, key)
				assert.Equal(envVars[key], value)
			}
		})
	}
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
