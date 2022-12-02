package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/conf"
	devenv "github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/dev/logs"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/print"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/builtins"
	deployconfig "github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/outputs"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/bufiox"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/ojson"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// Executor is an interface that contains methods for executing task code.
type Executor interface {
	Execute(ctx context.Context, config LocalRunConfig) (api.Outputs, error)
}

// LocalExecutor is an implementation of Executor that runs task code locally.
type LocalExecutor struct {
	BuiltinsClient *builtins.LocalBuiltinClient
}

func NewLocalExecutor(workingDir string) Executor {
	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{})
	builtinsClient, err := builtins.NewLocalClient(workingDir, goruntime.GOOS, goruntime.GOARCH, l)
	if err != nil {
		l.Error(err.Error())
		l.Warning("Local builtin execution is not supported on this machine. Builtin executions will error.")
		builtinsClient = nil
	}

	return &LocalExecutor{
		BuiltinsClient: builtinsClient,
	}
}

// LocalRunConfig is a struct that contains the necessary configs for running a task locally.
type LocalRunConfig struct {
	ID          string
	Name        string
	Kind        build.TaskKind
	KindOptions build.KindOptions
	ParamValues api.Values
	File        string
	Slug        string
	ParentRunID *string
	EnvVars     map[string]string
	ConfigVars  map[string]string
	AuthInfo    api.AuthInfoResponse

	LocalClient  *api.Client
	RemoteClient api.APIClient

	// Mapping from alias to resource
	AliasToResource map[string]resources.Resource

	IsBuiltin bool
	LogBroker logs.LogBroker
	PrintLogs bool
	// WorkingDir is where the studio is running
	WorkingDir string
}

type CmdConfig struct {
	cmd        *exec.Cmd
	closer     io.Closer
	entrypoint string
	runtime    runtime.Interface
}

var LogIDGen IDGenerator

// Cmd returns the command needed to execute the task locally
func (l *LocalExecutor) Cmd(ctx context.Context, runConfig LocalRunConfig) (CmdConfig, error) {
	if runConfig.IsBuiltin {
		if l.BuiltinsClient == nil {
			return CmdConfig{}, errors.New("builtins are not supported on this machine")
		}

		req, err := builtins.MarshalRequest(runConfig.Slug, runConfig.ParamValues)
		if err != nil {
			return CmdConfig{}, errors.New("invalid builtin request")
		}
		cmd, err := l.BuiltinsClient.Cmd(ctx, req)
		if err != nil {
			return CmdConfig{}, err
		}
		return CmdConfig{cmd: cmd, closer: l.BuiltinsClient.Closer}, nil
	}
	entrypoint, err := entrypointFrom(runConfig.File)
	if err != nil && err != definitions.ErrNoEntrypoint {
		// REST tasks don't have an entrypoint, and it's not needed
		return CmdConfig{}, err
	}
	r, err := runtime.Lookup(entrypoint, runConfig.Kind)
	if err != nil {
		return CmdConfig{}, errors.Wrapf(err, "unsupported file type: %s", filepath.Base(entrypoint))
	}

	if !r.SupportsLocalExecution() {
		logger.Warning("Local execution is not supported for this task (kind=%s)", runConfig.Kind)
		return CmdConfig{}, nil
	}

	cmds, closer, err := r.PrepareRun(ctx, logger.NewStdErrLogger(logger.StdErrLoggerOpts{}), runtime.PrepareRunOptions{
		Path:           entrypoint,
		ParamValues:    runConfig.ParamValues,
		KindOptions:    runConfig.KindOptions,
		TaskSlug:       runConfig.Slug,
		WorkingDir:     runConfig.WorkingDir,
		BuiltinsClient: l.BuiltinsClient,
	})
	if err != nil {
		return CmdConfig{}, err
	}

	cmd := exec.CommandContext(ctx, cmds[0], cmds[1:]...)
	return CmdConfig{
		cmd:        cmd,
		closer:     closer,
		entrypoint: entrypoint,
		runtime:    r,
	}, nil
}

func (l *LocalExecutor) Execute(ctx context.Context, config LocalRunConfig) (api.Outputs, error) {
	if config.KindOptions != nil {
		interpolatedKindOptions, err := interpolate(ctx, config, config.KindOptions)
		if err != nil {
			return api.Outputs{}, err
		}

		var ok bool
		if config.KindOptions, ok = interpolatedKindOptions.(map[string]interface{}); !ok {
			return api.Outputs{}, errors.Errorf("expected kind options after interpolation, got %T", config.KindOptions)
		}
	}

	cmdConfig, err := l.Cmd(ctx, config)
	if cmdConfig.closer != nil {
		defer cmdConfig.closer.Close()
	}
	if err != nil {
		return api.Outputs{}, err
	}
	cmd := cmdConfig.cmd
	r := cmdConfig.runtime
	entrypoint := cmdConfig.entrypoint
	logger.Log("%+v Locally running task %s (runID=%s).", logger.Yellow(time.Now().Format(logger.TimeFormatNoDate)), logger.Bold(config.Slug), logger.Gray(config.ID))

	logger.Debug("Running %s", logger.Bold(strings.Join(cmd.Args, " ")))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return api.Outputs{}, errors.Wrap(err, "stdout")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return api.Outputs{}, errors.Wrap(err, "stderr")
	}

	// cmd.Env defaults to os.Environ _only if empty_. Since we add
	// to it, we need to also set it to os.Environ.
	cmd.Env = os.Environ()
	// only non builtins have a runtime
	if r != nil {
		// Load environment variables from .env files
		// TODO: Remove support for .env files
		envVars, err := getDevEnvVars(r, entrypoint)
		if err != nil {
			return api.Outputs{}, err
		}

		if len(config.EnvVars) > 0 {
			envVars = config.EnvVars
		}

		if len(envVars) > 0 {
			result, err := interpolate(ctx, config, envVars)
			if err != nil {
				return api.Outputs{}, err
			}

			envVarsMap, ok := result.(map[string]interface{})
			if !ok {
				return api.Outputs{}, errors.Errorf("expected map of env vars (key=value pairs) after interpolation, got %T", envVarsMap)
			}
			for k, v := range envVarsMap {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	if cmd.Env, err = appendAirplaneEnvVars(cmd.Env, config); err != nil {
		return api.Outputs{}, errors.Wrap(err, "appending airplane-specific env vars")
	}
	if err := cmd.Start(); err != nil {
		return api.Outputs{}, errors.Wrap(err, "starting")
	}

	if config.LogBroker == nil {
		config.LogBroker = &logs.MockLogBroker{}
	}
	defer func() {
		config.LogBroker.Close()
	}()

	// mu guards o and chunks
	var mu sync.Mutex
	var o ojson.Value
	chunks := make(map[string]*strings.Builder)

	logParser := func(r io.Reader) error {
		scanner := bufiox.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			scanLogLine(config, line, &mu, &o, chunks)
			config.LogBroker.Record(api.LogItem{
				Timestamp: time.Now(),
				InsertID:  LogIDGen.Next(),
				Text:      line,
				Level:     "info",
			})
			if config.PrintLogs {
				logger.Log("[%s %s] %s", logger.Gray(config.Name), logger.Gray("log"), line)
			}
		}
		return errors.Wrap(scanner.Err(), "scanning logs")
	}

	eg := errgroup.Group{}
	eg.Go(func() error {
		return logParser(stdout)
	})
	eg.Go(func() error {
		return logParser(stderr)
	})

	if err = eg.Wait(); err != nil {
		return api.Outputs{}, err
	}

	err = cmd.Wait()
	outputs := api.Outputs(o)
	if config.PrintLogs {
		logger.Log("")
		logger.Log("%s for task %s:", logger.Gray("Output"), logger.Gray(config.Slug))
		print.Outputs(outputs)
	}
	logger.Log("%v Finished running task %s (runID=%s).", logger.Yellow(time.Now().Format(logger.TimeFormatNoDate)), logger.Bold(config.Slug), logger.Gray(config.ID))

	return outputs, err
}

func GetKindAndOptions(taskConfig discover.TaskConfig) (build.TaskKind, build.KindOptions, error) {
	kind, kindOptions, err := taskConfig.Def.GetKindAndOptions()
	if err != nil {
		return "", build.KindOptions{}, errors.Wrap(err, "getting kind and kind options")
	}

	buildConfig, err := taskConfig.Def.GetBuildConfig()
	if err != nil {
		return "", build.KindOptions{}, errors.Wrap(err, "getting build config")
	}
	if entrypointFunc, ok := buildConfig["entrypointFunc"]; ok {
		// Config as code depends on using the correct import name
		kindOptions["entrypointFunc"] = entrypointFunc
	}
	return kind, kindOptions, nil
}

func MaterializeEnvVars(taskConfig discover.TaskConfig, config *conf.DevConfig) (map[string]string, error) {
	envVars := map[string]string{}

	taskEnvVars, err := taskConfig.Def.GetEnv()
	if err != nil {
		return nil, err
	}

	// Add env vars from the airplane.yaml config file.
	kind, _, err := taskConfig.Def.GetKindAndOptions()
	if err != nil {
		return nil, err
	}
	entrypoint, err := taskConfig.Def.GetAbsoluteEntrypoint()
	if err != nil {
		return nil, err
	}
	r, err := runtime.Lookup(entrypoint, kind)
	if err != nil {
		return nil, err
	}
	root, err := r.Root(entrypoint)
	if err != nil {
		return nil, err
	}
	configFilePath := filepath.Join(root, deployconfig.FileName)
	hasExistingConfigFile := fsx.Exists(configFilePath)
	if hasExistingConfigFile {
		airplaneConfig, err := deployconfig.NewAirplaneConfigFromFile(configFilePath)
		if err != nil {
			return nil, err
		}
		for key, envVar := range airplaneConfig.EnvVars {
			if _, ok := taskEnvVars[key]; !ok {
				if taskEnvVars == nil {
					taskEnvVars = make(libapi.TaskEnv)
				}
				taskEnvVars[key] = envVar
			}
		}
	}

	for key, envVar := range taskEnvVars {
		if envVar.Value != nil {
			envVars[key] = *envVar.Value
		} else if envVar.Config != nil {
			if configVal, ok := config.ConfigVars[*envVar.Config]; !ok {
				logger.Warning("config %s is not defined in airplane.dev.yaml (referenced by env var %s)", *envVar.Config, key)
			} else {
				envVars[key] = configVal
			}
		}
	}
	return envVars, nil
}

func scanLogLine(config LocalRunConfig, line string, mu *sync.Mutex, o *ojson.Value, chunks map[string]*strings.Builder) {
	scanForErrors(config.RemoteClient, line)
	mu.Lock()
	defer mu.Unlock()
	parsed, err := outputs.Parse(chunks, line, outputs.ParseOptions{})
	if err != nil {
		logger.Error("[%s] %+v", logger.Gray("outputs"), err)
		return
	}
	if parsed != nil {
		err := outputs.ApplyOutputCommand(parsed, o)
		if err != nil {
			logger.Error("[%s] %+v", logger.Gray("outputs"), err)
			return
		}
	}
}

// getDevEnvVars will return a map of env vars, loading from .env and airplane.env
// files inside the task root.
//
// Env variables are first loaded by looking for any .env files between the root
// and entrypoint dir (inclusive). A second pass is done to look for airplane.env
// files. Env vars from successive files are merged in and overwrite duplicate keys.
func getDevEnvVars(r runtime.Interface, path string) (map[string]string, error) {
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

// Returns an absolute entrypoint.
func entrypointFrom(file string) (string, error) {
	format := definitions.GetTaskDefFormat(file)
	switch format {
	case definitions.DefFormatYAML, definitions.DefFormatJSON:
		return entrypointFromDefn(file)
	default:
		path, err := filepath.Abs(file)
		if err != nil {
			return "", errors.Wrapf(err, "absolute path of %s", file)
		}
		return path, nil
	}
}

func entrypointFromDefn(file string) (string, error) {
	dir, err := taskdir.Open(file)
	if err != nil {
		return "", err
	}
	defer dir.Close()

	def, err := dir.ReadDefinition()
	if err != nil {
		return "", err
	}

	return def.GetAbsoluteEntrypoint()
}

func appendAirplaneEnvVars(env []string, config LocalRunConfig) ([]string, error) {
	env = append(env, fmt.Sprintf("AIRPLANE_API_HOST=%s", config.LocalClient.HostURL()))
	env = append(env, "AIRPLANE_RESOURCES_VERSION=2")

	var runnerID, runnerEmail string
	if config.AuthInfo.User != nil {
		runnerID = config.AuthInfo.User.ID
		runnerEmail = config.AuthInfo.User.Email
	}

	var teamID string
	if config.AuthInfo.Team != nil {
		teamID = config.AuthInfo.Team.ID
	}

	// Environment variables documented in https://docs.airplane.dev/tasks/runtime-api-reference#environment-variables
	// We omit:
	// - AIRPLANE_REQUESTER_EMAIL
	// - AIRPLANE_REQUESTER_ID
	// - AIRPLANE_SESSION_ID
	// - AIRPLANE_TASK_REVISION_ID
	// - AIRPLANE_TRIGGER_ID
	// because there is no requester, session, task revision, or triggers in the context of local dev.
	env = append(env,
		fmt.Sprintf("AIRPLANE_ENV_ID=%s", devenv.LocalEnvID),
		fmt.Sprintf("AIRPLANE_ENV_SLUG=%s", devenv.LocalEnvID),
		fmt.Sprintf("AIRPLANE_RUN_ID=%s", config.ID),
		fmt.Sprintf("AIRPLANE_PARENT_RUN_ID=%s", pointers.ToString(config.ParentRunID)),
		fmt.Sprintf("AIRPLANE_RUNNER_EMAIL=%s", runnerEmail),
		fmt.Sprintf("AIRPLANE_RUNNER_ID=%s", runnerID),
		"AIRPLANE_RUNTIME=dev",
		fmt.Sprintf("AIRPLANE_TASK_ID=%s", config.Slug), // For local dev, we use the task's slug as its id.
		fmt.Sprintf("AIRPLANE_TEAM_ID=%s", teamID),
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

func interpolate(ctx context.Context, cfg LocalRunConfig, value any) (any, error) {
	resp, err := cfg.RemoteClient.EvaluateTemplate(ctx, libapi.EvaluateTemplateRequest{
		Value:       value,
		RunID:       cfg.ID,
		Env:         devenv.NewLocalEnv(),
		Resources:   cfg.AliasToResource,
		Configs:     cfg.ConfigVars,
		ParamValues: cfg.ParamValues,
	})
	if err != nil {
		var apiErr api.Error
		if errors.As(err, &apiErr) {
			return nil, errors.New(apiErr.Message)
		}
		return nil, err
	}

	return resp.Value, nil
}
