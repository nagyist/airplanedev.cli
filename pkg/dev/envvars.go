package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	libapi "github.com/airplanedev/cli/pkg/api"
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/conf"
	devenv "github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
)

type GetEnvVarsForViewConfig struct {
	ViewEnvVars      map[string]libapi.EnvVarValue
	DevConfigEnvVars map[string]string
	ConfigVars       map[string]devenv.ConfigWithEnv
	FallbackEnvSlug  string
	AuthInfo         api.AuthInfoResponse
	Name             string
	Slug             string
	ViewURL          string
	APIHeaders       map[string]string
}

func GetEnvVarsForView(
	ctx context.Context,
	remoteClient api.APIClient,
	config GetEnvVarsForViewConfig,
) (map[string]string, error) {
	envVars := make(map[string]string)

	// Note that views do not pull env vars from dotenv files.
	unmaterializedEnvVars := applyEnvVarFileOverrides(config.ViewEnvVars, config.DevConfigEnvVars, nil, "")

	// Materialize configs to values.
	materializedEnvVars, err := materializeEnvVars(
		ctx,
		remoteClient,
		unmaterializedEnvVars,
		config.ConfigVars,
		config.FallbackEnvSlug,
	)
	if err != nil {
		return nil, err
	}
	maps.Copy(envVars, materializedEnvVars)

	// We skip interpolating JSTs. We may add this in the future, but this
	// is probably not useful for views.

	airplaneEnvVars, err := getBuiltInViewEnvVars(config)
	if err != nil {
		return nil, err
	}
	maps.Copy(envVars, airplaneEnvVars)

	return envVars, nil
}

// getEnvVars sets the environment variables for the run command.
func getEnvVars(
	ctx context.Context,
	config LocalRunConfig,
	r runtime.Interface,
	entrypoint string,
	interpolateRequest libapi.EvaluateTemplateRequest,
) ([]string, error) {
	env := filteredSystemEnvVars()

	// only non builtins have a runtime
	if r != nil {
		// Collect all environment variables for the current run.
		runEnvVars := applyEnvVarFileOverrides(config.TaskEnvVars, config.EnvVars, r, entrypoint)

		// Convert configs to values.
		materializedEnvVars, err := materializeEnvVars(
			ctx,
			config.RemoteClient,
			runEnvVars,
			config.ConfigVars,
			config.FallbackEnvSlug,
		)
		if err != nil {
			return nil, err
		}

		// Interpolate any JSTs in environment variables.
		if len(materializedEnvVars) > 0 {
			result, err := interpolate(ctx, config.RemoteClient, interpolateRequest, StrictModeOn, materializedEnvVars)
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

	airplaneEnvVars, err := getBuiltInTaskEnvVars(config)
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
	fallbackEnvSlug string,
) (map[string]string, error) {
	envVars := make(map[string]string, len(taskEnvVars))
	for key, envVar := range taskEnvVars {
		if envVar.Value != nil {
			envVars[key] = *envVar.Value
		} else if envVar.Config != nil {
			if configVar, ok := configVars[*envVar.Config]; !ok {
				errMessage := fmt.Sprintf("Config var %s not defined in airplane.dev.yaml", *envVar.Config)
				if fallbackEnvSlug != "" {
					errMessage += fmt.Sprintf(" or remotely in env %s", fallbackEnvSlug)
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

// applyEnvVarFileOverrides applies any overrides from dotenv or dev config files to the entity's environment variables.
func applyEnvVarFileOverrides(entityEnvVars map[string]libapi.EnvVarValue, devConfigEnvVars map[string]string,
	r runtime.Interface, entrypoint string) libapi.TaskEnv {
	envVars := make(map[string]libapi.EnvVarValue)
	maps.Copy(envVars, entityEnvVars)

	if r != nil {
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
	}

	// Env vars declared in the dev config file take precedence to those declared in the task definition or .env
	// files
	for k, v := range devConfigEnvVars {
		v := v
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

// getBuiltInTaskEnvVars gets all built in task environment variables.
func getBuiltInTaskEnvVars(config LocalRunConfig) ([]string, error) {
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
		fmt.Sprintf("AIRPLANE_RUN_ID=%s", config.ID),
		fmt.Sprintf("AIRPLANE_PARENT_RUN_ID=%s", pointers.ToString(config.ParentRunID)),
		fmt.Sprintf("AIRPLANE_RUNNER_EMAIL=%s", runnerEmail),
		fmt.Sprintf("AIRPLANE_RUNNER_ID=%s", runnerID),
		"AIRPLANE_RUNTIME=dev",
		fmt.Sprintf("AIRPLANE_TASK_ID=%s", config.Slug), // For local dev, we use the task's slug as its id.
		fmt.Sprintf("AIRPLANE_TASK_SLUG=%s", config.Slug),
		fmt.Sprintf("AIRPLANE_TASK_NAME=%s", config.Name),
		fmt.Sprintf("AIRPLANE_TEAM_ID=%s", teamID),
		fmt.Sprintf("AIRPLANE_RUNNER_NAME=%s", runnerName),
		fmt.Sprintf("AIRPLANE_TASK_URL=%s", taskURL),
		fmt.Sprintf("AIRPLANE_RUN_URL=%s", runURL),
	)
	for k, v := range getCommonEnvVars() {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

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

	if config.TunnelToken != nil {
		env = append(env, fmt.Sprintf("AIRPLANE_TUNNEL_TOKEN=%s", *config.TunnelToken))
	}

	return env, nil
}

// getBuiltInViewEnvVars gets all built in view environment variables.
func getBuiltInViewEnvVars(config GetEnvVarsForViewConfig) (map[string]string, error) {
	env := make(map[string]string)

	var userID, userEmail, userName string
	if config.AuthInfo.User != nil {
		userID = config.AuthInfo.User.ID
		userEmail = config.AuthInfo.User.Email
		userName = config.AuthInfo.User.Name
	}

	var teamID string
	if config.AuthInfo.Team != nil {
		teamID = config.AuthInfo.Team.ID
	}

	env["AIRPLANE_USER_EMAIL"] = userEmail
	env["AIRPLANE_USER_ID"] = userID
	env["AIRPLANE_USER_NAME"] = userName

	env["AIRPLANE_VIEW_ID"] = config.Slug // For local dev, we use the view's slug as its id.
	env["AIRPLANE_VIEW_SLUG"] = config.Slug
	env["AIRPLANE_VIEW_NAME"] = config.Name
	env["AIRPLANE_VIEW_URL"] = config.ViewURL

	env["AIRPLANE_TEAM_ID"] = teamID

	if len(config.APIHeaders) > 0 {
		serializedHeaders, err := json.Marshal(config.APIHeaders)
		if err != nil {
			return nil, err
		}
		env["AIRPLANE_API_HEADERS"] = string(serializedHeaders)
	}

	maps.Copy(env, getCommonEnvVars())

	return env, nil
}

// getCommonEnvVars gets environment variables that are common to both tasks and views.
func getCommonEnvVars() map[string]string {
	return map[string]string{
		"AIRPLANE_ENV_ID":         devenv.StudioEnvID,
		"AIRPLANE_ENV_SLUG":       devenv.StudioEnvID,
		"AIRPLANE_ENV_NAME":       devenv.StudioEnvID,
		"AIRPLANE_ENV_IS_DEFAULT": "true", // For local dev, there is one env.
	}
}

var allowedSystemEnvVars = map[string]bool{
	"HOME": true, // Used by the snowflake client to identify the user's home directory for caching.
	"PATH": true, // Allow airplane dev to access available executables
}

// filteredSystemEnvVars returns a list of environment variables from the user's system that can be passed to the task.
func filteredSystemEnvVars() []string {
	var filtered []string
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		k, v := parts[0], parts[1]
		if _, ok := allowedSystemEnvVars[k]; ok {
			filtered = append(filtered, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return filtered
}

// GetDiscoveryEnvVars get environment variables that are available during discovery.
func GetDiscoveryEnvVars(devConfig *conf.DevConfig) []string {
	discoveryEnvVars := getCommonEnvVars()
	discoveryEnvVars["AIRPLANE_RUNTIME"] = "build" // emulates builder behavior
	envVars := generateEnvVarList(discoveryEnvVars)

	if devConfig != nil {
		envVars = append(envVars, generateEnvVarList(devConfig.EnvVars)...)
	}

	return envVars
}

func generateEnvVarList(envVarMap map[string]string) []string {
	var envVarList []string
	for key, value := range envVarMap {
		envVarList = append(envVarList, fmt.Sprintf("%s=%s", key, value))
	}
	return envVarList
}
