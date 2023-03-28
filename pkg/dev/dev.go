package dev

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	devenv "github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/dev/logs"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/print"
	libapi "github.com/airplanedev/lib/pkg/api"
	libhttp "github.com/airplanedev/lib/pkg/api/http"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/builtins"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/outputs"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/bufiox"
	"github.com/airplanedev/ojson"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// Executor is an interface that contains methods for executing task code.
type Executor interface {
	Execute(ctx context.Context, config LocalRunConfig) (api.Outputs, error)
	Refresh() error
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
	Kind        buildtypes.TaskKind
	KindOptions buildtypes.KindOptions
	ParamValues api.Values
	File        string
	Slug        string
	ParentRunID *string
	AuthInfo    api.AuthInfoResponse

	LocalClient     *api.Client
	RemoteClient    api.APIClient
	FallbackEnvSlug string
	TunnelToken     *string

	// Mapping from alias to resource
	AliasToResource map[string]resources.Resource

	ConfigAttachments []libapi.ConfigAttachment
	ConfigVars        map[string]devenv.ConfigWithEnv
	EnvVars           map[string]string
	TaskEnvVars       libapi.TaskEnv

	IsBuiltin bool
	LogBroker logs.LogBroker
	PrintLogs bool
	// WorkingDir is where the studio is running
	WorkingDir string

	StudioURL url.URL
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
		RunID:          runConfig.ID,
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
	configVars, err := materializeConfigAttachments(
		ctx,
		config.RemoteClient,
		config.ConfigAttachments, config.ConfigVars,
		config.FallbackEnvSlug,
	)
	if err != nil {
		return api.Outputs{}, err
	}

	baseInterpolateRequest := baseEvaluateTemplateRequest(config, configVars)

	if config.KindOptions != nil {
		interpolatedKindOptions, err := interpolate(ctx, config.RemoteClient, baseInterpolateRequest, config.KindOptions)
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

	if cmd.Env, err = getEnvVars(ctx, config, r, entrypoint, baseInterpolateRequest); err != nil {
		return api.Outputs{}, err
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

func (l *LocalExecutor) Refresh() error {
	logger.Debug("Refreshing local executor")
	if l.BuiltinsClient != nil {
		_, err := l.BuiltinsClient.Download()
		if err != nil {
			return errors.Wrap(err, "downloading builtins binary")
		}
	}

	return nil
}

func GetKindAndOptions(taskConfig discover.TaskConfig) (buildtypes.TaskKind, buildtypes.KindOptions, error) {
	kind, kindOptions, err := taskConfig.Def.GetKindAndOptions()
	if err != nil {
		return "", buildtypes.KindOptions{}, errors.Wrap(err, "getting kind and kind options")
	}

	buildConfig, err := taskConfig.Def.GetBuildConfig()
	if err != nil {
		return "", buildtypes.KindOptions{}, errors.Wrap(err, "getting build config")
	}
	if entrypointFunc, ok := buildConfig["entrypointFunc"]; ok {
		// Config as code depends on using the correct import name
		kindOptions["entrypointFunc"] = entrypointFunc
	}
	return kind, kindOptions, nil
}

// materializeConfigAttachments returns the configs that are attached to a task
func materializeConfigAttachments(
	ctx context.Context,
	remoteClient api.APIClient,
	attachments []libapi.ConfigAttachment,
	configs map[string]devenv.ConfigWithEnv,
	fallbackEnvSlug string,
) (map[string]string, error) {
	configAttachments := map[string]string{}
	for _, a := range attachments {
		cfg, ok := configs[a.NameTag]
		if !ok {
			errMessage := fmt.Sprintf("Config var %s not defined in airplane.dev.yaml", a.NameTag)
			if fallbackEnvSlug != "" {
				errMessage += fmt.Sprintf(" or remotely in env %s", fallbackEnvSlug)
			}
			errMessage += ". Please use the configs tab on the left to add it."
			return nil, errors.New(errMessage)
		}

		var err error
		if configAttachments[a.NameTag], err = getConfigValue(ctx, remoteClient, cfg); err != nil {
			return nil, err
		}
	}
	return configAttachments, nil
}

func getConfigValue(ctx context.Context, remoteClient api.APIClient, config devenv.ConfigWithEnv) (string, error) {
	if config.Remote && config.IsSecret {
		resp, err := remoteClient.GetConfig(ctx, api.GetConfigRequest{
			Name:       config.Name,
			ShowSecret: true,
			EnvSlug:    config.Env.Slug,
		})
		if err != nil {
			return "", errors.Wrapf(err, "decrypting remote config %s", config.Name)
		}

		return resp.Config.Value, nil
	}

	return config.Value, nil
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

func baseEvaluateTemplateRequest(cfg LocalRunConfig, configVars map[string]string) libapi.EvaluateTemplateRequest {
	return libapi.EvaluateTemplateRequest{
		RunID:       cfg.ID,
		Env:         devenv.NewLocalEnv(),
		Resources:   cfg.AliasToResource,
		Configs:     configVars,
		ParamValues: cfg.ParamValues,
	}
}

func interpolate(ctx context.Context, remoteClient api.APIClient, baseRequest libapi.EvaluateTemplateRequest, value any) (any, error) {
	resp, err := remoteClient.EvaluateTemplate(ctx, libapi.EvaluateTemplateRequest{
		Value:       value,
		RunID:       baseRequest.RunID,
		Env:         baseRequest.Env,
		Resources:   baseRequest.Resources,
		Configs:     baseRequest.Configs,
		ParamValues: baseRequest.ParamValues,
	})
	if err != nil {
		var errsc libhttp.ErrStatusCode
		if errors.As(err, &errsc) {
			return nil, errors.New(errsc.Msg)
		}
		return nil, err
	}

	return resp.Value, nil
}
