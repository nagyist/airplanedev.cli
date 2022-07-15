package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/print"
	"github.com/airplanedev/cli/pkg/resource"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/outputs"
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
type LocalExecutor struct{}

// LocalRunConfig is a struct that contains the necessary configs for running a task locally.
type LocalRunConfig struct {
	Name        string
	Kind        build.TaskKind
	KindOptions build.KindOptions
	ParamValues api.Values
	Port        int
	Root        *cli.Config
	File        string
	Slug        string
	EnvSlug     string
	Env         map[string]string
	// Mapping from alias to resource
	Resources map[string]resource.Resource
}

func (l *LocalExecutor) Execute(ctx context.Context, config LocalRunConfig) (api.Outputs, error) {
	entrypoint, err := entrypointFrom(config.File)
	if err == definitions.ErrNoEntrypoint {
		logger.Warning("Local execution is not supported for this task (kind=%s)", config.Kind)
		return api.Outputs{}, nil
	} else if err != nil {
		return api.Outputs{}, err
	}

	r, err := runtime.Lookup(entrypoint, config.Kind)
	if err != nil {
		return api.Outputs{}, errors.Wrapf(err, "unsupported file type: %s", filepath.Base(entrypoint))
	}

	if !r.SupportsLocalExecution() {
		logger.Warning("Local execution is not supported for this task (kind=%s)", config.Kind)
		return api.Outputs{}, nil
	}

	print.BoxPrint(fmt.Sprintf("Locally running task [%s] %s", config.Name, config.Root.Client.TaskURL(config.Slug, config.EnvSlug)))
	logger.Log("")

	cmds, closer, err := r.PrepareRun(ctx, logger.NewStdErrLogger(logger.StdErrLoggerOpts{}), runtime.PrepareRunOptions{
		Path:        entrypoint,
		ParamValues: config.ParamValues,
		KindOptions: config.KindOptions,
	})
	if err != nil {
		return api.Outputs{}, err
	}
	if closer != nil {
		defer closer.Close()
	}

	cmd := exec.CommandContext(ctx, cmds[0], cmds[1:]...)
	logger.Debug("Running %s", logger.Bold(strings.Join(cmd.Args, " ")))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return api.Outputs{}, errors.Wrap(err, "stdout")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return api.Outputs{}, errors.Wrap(err, "stderr")
	}

	// Load environment variables from .env files:
	env, err := getDevEnv(r, entrypoint)
	if err != nil {
		return api.Outputs{}, err
	}

	// If environment variables are specified in the dev config file, use those instead
	if len(config.Env) > 0 {
		env = config.Env
	}

	// cmd.Env defaults to os.Environ _only if empty_. Since we add
	// to it, we need to also set it to os.Environ.
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	if cmd.Env, err = appendAirplaneEnvVars(cmd.Env, config); err != nil {
		return api.Outputs{}, errors.Wrap(err, "appending airplane-specific env vars")
	}

	if err := cmd.Start(); err != nil {
		return api.Outputs{}, errors.Wrap(err, "starting")
	}

	// mu guards o and chunks
	var mu sync.Mutex
	var o ojson.Value
	chunks := make(map[string]*strings.Builder)

	logParser := func(r io.Reader) error {
		scanner := bufiox.NewScanner(r)
		for scanner.Scan() {
			// TODO: Extract this logic into separate function so we can do defer mu.Unlock()
			line := scanner.Text()
			scanForErrors(config.Root, line)
			mu.Lock()
			parsed, err := outputs.Parse(chunks, line, outputs.ParseOptions{})
			if err != nil {
				mu.Unlock()
				logger.Error("[%s] %+v", logger.Gray("outputs"), err)
				continue
			}
			if parsed != nil {
				err := outputs.ApplyOutputCommand(parsed, &o)
				mu.Unlock()
				if err != nil {
					logger.Error("[%s] %+v", logger.Gray("outputs"), err)
					continue
				}
			} else {
				mu.Unlock()
			}

			logger.Log("[%s %s] %s", logger.Gray(config.Name), logger.Gray("log"), line)
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

	if err := eg.Wait(); err != nil {
		return api.Outputs{}, err
	}

	if err := cmd.Wait(); err != nil {
		return api.Outputs{}, err
	}

	outputs := api.Outputs(o)
	logger.Log("")
	logger.Log("%s for task %s:", logger.Gray("Output"), logger.Gray(config.Slug))
	print.Outputs(outputs)

	logger.Log("")
	print.BoxPrint(fmt.Sprintf("Finished running task [%s]", config.Name))
	logger.Log("")

	analytics.Track(config.Root, "Run Executed Locally", map[string]interface{}{
		"kind":         config.Kind,
		"task_slug":    config.Slug,
		"task_name":    config.Name,
		"env_slug":     config.EnvSlug,
		"num_params":   len(config.ParamValues),
		"num_env_vars": len(cmd.Env),
	}, analytics.TrackOpts{
		SkipSlack: true,
	})

	return outputs, nil
}

// getDevEnv will return a map of env vars, loading from .env and airplane.env
// files inside the task root.
//
// Env variables are first loaded by looking for any .env files between the root
// and entrypoint dir (inclusive). A second pass is done to look for airplane.env
// files. Env vars from successive files are merged in and overwrite duplicate keys.
func getDevEnv(r runtime.Interface, path string) (map[string]string, error) {
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
	env = append(env, fmt.Sprintf("AIRPLANE_API_HOST=http://127.0.0.1:%d", config.Port))
	env = append(env, "AIRPLANE_RUNTIME=dev")

	serialized, err := json.Marshal(config.Resources)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling resources")
	}

	env = append(env, fmt.Sprintf("AIRPLANE_RESOURCES=%s", string(serialized)))
	return env, nil
}
