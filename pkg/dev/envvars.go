package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/api"
	devenv "github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

// getEnvVars sets the environment variables for the run command.
func getEnvVars(
	ctx context.Context,
	config LocalRunConfig,
	r runtime.Interface,
	entrypoint string,
	interpolateRequest libapi.EvaluateTemplateRequest,
) ([]string, error) {
	// cmd.Env defaults to os.Environ _only if empty_. Since we add
	// to it, we need to also set it to os.Environ.
	env := os.Environ()
	// only non builtins have a runtime
	if r != nil {
		// Collect all environment variables for the current run.
		runEnvVars := applyEnvVarOverrides(config, r, entrypoint)

		// Convert configs to values.
		materializedEnvVars, err := materializeEnvVars(
			ctx,
			config.RemoteClient,
			runEnvVars,
			config.ConfigVars,
			config.UseFallbackEnv,
		)
		if err != nil {
			return nil, err
		}

		// Interpolate any JSTs in environment variables.
		if len(materializedEnvVars) > 0 {
			result, err := interpolate(ctx, config.RemoteClient, interpolateRequest, materializedEnvVars)
			if err != nil {
				return nil, err
			}

			envVarsMap, ok := result.(map[string]interface{})
			if !ok {
				return nil, errors.Errorf("expected map of env vars (key=value pairs) after interpolation, got %T", envVarsMap)
			}
			for k, v := range envVarsMap {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	airplaneEnvVars, err := getAirplaneEnvVars(config)
	if err != nil {
		return nil, err
	}

	env = append(env, airplaneEnvVars...)

	return env, nil
}

// materializeEnvVars materializes any environment variables that derive their values from local or remote config
// variables.
func materializeEnvVars(
	ctx context.Context,
	remoteClient api.APIClient,
	taskEnvVars libapi.TaskEnv,
	configVars map[string]devenv.ConfigWithEnv,
	useFallbackEnv bool,
) (map[string]string, error) {
	envVars := make(map[string]string, len(taskEnvVars))
	for key, envVar := range taskEnvVars {
		if envVar.Value != nil {
			envVars[key] = *envVar.Value
		} else if envVar.Config != nil {
			if configVar, ok := configVars[*envVar.Config]; !ok {
				errMessage := fmt.Sprintf("Config var %s not defined in airplane.dev.yaml", *envVar.Config)
				if useFallbackEnv {
					errMessage += " or remotely in fallback env"
				}
				errMessage += fmt.Sprintf(" (referenced by env var %s). Please use the configs tab on the left to add it.", key)
				return nil, errors.New(errMessage)
			} else {
				var err error
				if envVars[key], err = getConfigValue(ctx, remoteClient, configVar); err != nil {
					return nil, err
				}
			}
		}
	}
	return envVars, nil
}

// applyEnvVarOverrides applies any overrides from dotenv or dev config files to the task's environment variables.
func applyEnvVarOverrides(config LocalRunConfig, r runtime.Interface, entrypoint string) libapi.TaskEnv {
	envVars := libapi.TaskEnv{}
	for k, v := range config.TaskEnvVars {
		envVars[k] = v
	}

	// Load environment variables from .env files
	// TODO: Deprecate support for .env files
	dotEnvEnvVars, err := getDotEnvEnvVars(r, entrypoint)
	if err != nil {
		return nil
	}
	// Env vars declared in .env files take precedence to those declared in the task definition.
	for k, v := range dotEnvEnvVars {
		v := v // capture loop variable
		envVars[k] = libapi.EnvVarValue{
			Value: &v,
		}
	}

	// Env vars declared in the dev config file take precedence to those declared in the task definition or .env
	// files
	for k, v := range config.EnvVars {
		v := v // capture loop variable
		envVars[k] = libapi.EnvVarValue{
			Value: &v,
		}
	}

	return envVars
}

// getDotEnvEnvVars will return a map of env vars from .env and airplane.env
// files inside the task root.
//
// Env variables are first loaded by looking for any .env files between the root
// and entrypoint dir (inclusive). A second pass is done to look for airplane.env
// files. Env vars from successive files are merged in and overwrite duplicate keys.
func getDotEnvEnvVars(r runtime.Interface, path string) (map[string]string, error) {
	root, err := r.Root(path)
	if err != nil {
		return nil, err
	}

	// dotenvs will contain a list of .env file paths that should be read.
	//
	// They will be loaded in order, with later .env files overwriting values
	// from earlier .env files.
	dotenvs := []string{}

	// Loop through directories from [workdir, root] inclusive, in reverse
	// order.
	dirs := []string{}
	for dir := filepath.Dir(path); dir != filepath.Dir(root); dir = filepath.Dir(dir) {
		dirs = append([]string{dir}, dirs...)
	}

	for _, file := range []string{".env", "airplane.env"} {
		for _, dir := range dirs {
			fp := filepath.Join(dir, file)
			if fsx.Exists(fp) {
				logger.Debug("Loading env vars from %s", logger.Bold(fp))
				dotenvs = append(dotenvs, fp)
			}
		}
	}

	if len(dotenvs) == 0 {
		return nil, nil
	}

	env, err := godotenv.Read(dotenvs...)
	return env, errors.Wrap(err, "reading .env")
}

// appendAirplaneEnvVars appends Airplane-specific environment variables to the given environment variables slice.
func getAirplaneEnvVars(config LocalRunConfig) ([]string, error) {
	var env []string

	env = append(env, fmt.Sprintf("AIRPLANE_API_HOST=%s", config.LocalClient.HostURL()))
	env = append(env, "AIRPLANE_RESOURCES_VERSION=2")

	var runnerID, runnerEmail, runnerName string
	if config.AuthInfo.User != nil {
		runnerID = config.AuthInfo.User.ID
		runnerEmail = config.AuthInfo.User.Email
		runnerName = config.AuthInfo.User.Name
	}

	var teamID string
	if config.AuthInfo.Team != nil {
		teamID = config.AuthInfo.Team.ID
	}
	apiPort := config.LocalClient.AppURL().Port()
	taskURL := utils.StudioURL(config.StudioURL.Host, apiPort, "/task/"+config.Slug)
	runURL := utils.StudioURL(config.StudioURL.Host, apiPort, "/runs/"+config.ID)

	// Environment variables documented in https://docs.airplane.dev/tasks/runtime-api-reference#environment-variables
	// We omit:
	// - AIRPLANE_REQUESTER_EMAIL
	// - AIRPLANE_REQUESTER_ID
	// - AIRPLANE_SESSION_ID
	// - AIRPLANE_TASK_REVISION_ID
	// - AIRPLANE_TRIGGER_ID
	// because there is no requester, session, task revision, or triggers in the context of local dev.
	env = append(env,
		fmt.Sprintf("AIRPLANE_ENV_ID=%s", devenv.StudioEnvID),
		fmt.Sprintf("AIRPLANE_ENV_SLUG=%s", devenv.StudioEnvID),
		fmt.Sprintf("AIRPLANE_ENV_NAME=%s", devenv.StudioEnvID),
		fmt.Sprintf("AIRPLANE_ENV_IS_DEFAULT=%v", true), // For local dev, there is one env.

		fmt.Sprintf("AIRPLANE_RUN_ID=%s", config.ID),
		fmt.Sprintf("AIRPLANE_PARENT_RUN_ID=%s", pointers.ToString(config.ParentRunID)),
		fmt.Sprintf("AIRPLANE_RUNNER_EMAIL=%s", runnerEmail),
		fmt.Sprintf("AIRPLANE_RUNNER_ID=%s", runnerID),
		"AIRPLANE_RUNTIME=dev",
		fmt.Sprintf("AIRPLANE_TASK_ID=%s", config.Slug), // For local dev, we use the task's slug as its id.
		fmt.Sprintf("AIRPLANE_TASK_NAME=%s", config.Name),
		fmt.Sprintf("AIRPLANE_TEAM_ID=%s", teamID),
		fmt.Sprintf("AIRPLANE_RUNNER_NAME=%s", runnerName),
		fmt.Sprintf("AIRPLANE_TASK_URL=%s", taskURL),
		fmt.Sprintf("AIRPLANE_RUN_URL=%s", runURL),
	)

	token, err := GenerateInsecureAirplaneToken(AirplaneTokenClaims{
		RunID: config.ID,
	})
	if err != nil {
		return nil, err
	}
	env = append(env, fmt.Sprintf("AIRPLANE_TOKEN=%s", token))

	serialized, err := json.Marshal(config.AliasToResource)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling resources")
	}
	env = append(env, fmt.Sprintf("AIRPLANE_RESOURCES=%s", string(serialized)))
	return env, nil
}