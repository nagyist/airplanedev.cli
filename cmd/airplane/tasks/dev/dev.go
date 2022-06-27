package dev

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DataDog/temporalite"
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	viewsdev "github.com/airplanedev/cli/cmd/airplane/views/dev"
	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/params"
	"github.com/airplanedev/cli/pkg/print"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/taskdir"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/outputs"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/bufiox"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/ojson"
	"github.com/cenkalti/backoff/v4"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/server/common/log"
	"golang.org/x/sync/errgroup"

	uiserver "github.com/temporalio/ui-server/server"
	uiconfig "github.com/temporalio/ui-server/server/config"
	uiserveroptions "github.com/temporalio/ui-server/server/server_options"
)

type config struct {
	root      *cli.Config
	fileOrDir string
	args      []string
	envSlug   string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = config{root: c}

	cmd := &cobra.Command{
		Use:   "dev ./path/to/file",
		Short: "Locally run a task",
		Long:  "Locally runs a task, optionally with specific parameters.",
		Example: heredoc.Doc(`
			airplane dev ./task.js [-- <parameters...>]
			airplane dev ./task.ts [-- <parameters...>]
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			// TODO: update the `dev` command to work w/out internet access
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.HasPrefix(args[0], "-") {
				wd, err := os.Getwd()
				if err != nil {
					return errors.Wrap(err, "error determining current working directory")

				}
				cfg.fileOrDir = wd
				cfg.args = args
			} else {
				cfg.fileOrDir = args[0]
				cfg.args = args[1:]
			}

			return run(cmd.Root().Context(), cfg)
		},
	}

	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment to query. Defaults to your team's default environment.")

	return cmd
}

func run(ctx context.Context, cfg config) error {
	if !fsx.Exists(cfg.fileOrDir) {
		return errors.Errorf("Unable to open: %s", cfg.fileOrDir)
	}

	isWorkflow := true
	namespace := "airplane_dev"
	taskQueue := "taskRevisionDev"

	if isWorkflow {
		temporalLogger := log.NewZapLogger(log.BuildZapLogger(log.Config{
			Stdout:     true,
			Level:      "fatal",
			OutputFile: "",
		}))

		server, err := temporalite.NewServer(
			temporalite.WithFrontendPort(7333),
			temporalite.WithPersistenceDisabled(),
			temporalite.WithLogger(temporalLogger),
			temporalite.WithUI(uiserver.NewServer(uiserveroptions.WithConfigProvider(&uiconfig.Config{
				TemporalGRPCAddress: ":7333",
				Host:                "localhost",
				Port:                8333,
				EnableUI:            true,
			}))),
		)
		if err != nil {
			return err
		}

		//defer server.Stop()

		if err := server.Start(); err != nil {
			return err
		}

		fmt.Println("server started")

		temporalClient, err := client.Dial(client.Options{
			HostPort: "localhost:7333",
		})

		retentionPeriod := time.Hour
		_, err = temporalClient.WorkflowService().RegisterNamespace(
			ctx,
			&workflowservice.RegisterNamespaceRequest{
				Namespace:                        namespace,
				Description:                      "namespace for airplane dev",
				WorkflowExecutionRetentionPeriod: &retentionPeriod,
			},
		)
		if err != nil {
			return errors.Wrap(err, "creating namespace")
		}

		b := &backoff.ExponentialBackOff{
			InitialInterval:     backoff.DefaultInitialInterval,
			RandomizationFactor: backoff.DefaultRandomizationFactor,
			Multiplier:          backoff.DefaultMultiplier,
			MaxInterval:         5 * time.Second,
			MaxElapsedTime:      30 * time.Second,
			Stop:                backoff.Stop,
			Clock:               backoff.SystemClock,
		}
		b.Reset()

		// Wait until the namespace is available; this can take a little while, so wrapping in a backoff
		if err := backoff.Retry(
			func() error {
				_, err := temporalClient.WorkflowService().DescribeNamespace(
					ctx,
					&workflowservice.DescribeNamespaceRequest{
						Namespace: namespace,
					},
				)
				return err
			},
			b,
		); err != nil {
			return err
		}

		type AirplaneArgs struct {
			// TODO: Don't pass JWT in args
			TaskRevisionEnvVarNames []string
			RuntimeEnv              map[string]string
		}

		namespaceClient, err := client.Dial(client.Options{
			HostPort:  "localhost:7333",
			Namespace: namespace,
		})
		if err != nil {
			return err
		}
		_, err = namespaceClient.ExecuteWorkflow(
			ctx,
			// We map:
			// (1) airplane run id -> temporal workflow id
			// (2) airplane task revision id -> temporal queue
			//
			// Thus, all runs for a given revision will share the same queue.
			client.StartWorkflowOptions{
				ID:                    "someID",
				TaskQueue:             taskQueue,
				WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
			},
			// This must match the name of the wrapper function in the SDK shim
			"__airplaneEntrypoint",
			[]string{"{}"},
			AirplaneArgs{
				TaskRevisionEnvVarNames: []string{},
				RuntimeEnv: map[string]string{
					"AIRPLANE_RUNTIME": "workflow",
				},
			},
		)
	}

	fileInfo, err := os.Stat(cfg.fileOrDir)
	if err != nil {
		return errors.Wrapf(err, "describing %s", cfg.fileOrDir)
	}

	if cfg.root.Dev && fileInfo.IsDir() && viewsdev.IsView(cfg.fileOrDir) == nil {
		// Switch to devving a view.
		return viewsdev.Run(ctx, viewsdev.Config{
			Root:    cfg.root,
			Dir:     cfg.fileOrDir,
			Args:    cfg.args,
			EnvSlug: cfg.envSlug,
		})
	}

	if fileInfo.IsDir() {
		return errors.Errorf("%s is a directory", cfg.fileOrDir)
	}

	taskInfo, err := getTaskInfo(ctx, cfg)
	if err != nil {
		return errors.Wrap(err, "getting task info")
	}

	entrypoint, err := entrypointFrom(cfg.fileOrDir)
	if err == definitions.ErrNoEntrypoint {
		logger.Warning("Local execution is not supported for this task (kind=%s)", taskInfo.kind)
		return nil
	} else if err != nil {
		return err
	}

	r, err := runtime.Lookup(entrypoint, taskInfo.kind)
	if err != nil {
		return errors.Wrapf(err, "unsupported file type: %s", filepath.Base(entrypoint))
	}

	if !r.SupportsLocalExecution() {
		logger.Warning("Local execution is not supported for this task (kind=%s)", taskInfo.kind)
		return nil
	}

	paramValues, err := params.CLI(cfg.args, taskInfo.name, taskInfo.parameters)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}

	logger.Log("Locally running %s task %s", logger.Bold(taskInfo.name), logger.Gray("("+cfg.root.Client.TaskURL(taskInfo.slug, cfg.envSlug)+")"))
	logger.Log("")

	cmds, closer, err := r.PrepareRun(ctx, logger.NewStdErrLogger(logger.StdErrLoggerOpts{}), runtime.PrepareRunOptions{
		Path:        entrypoint,
		ParamValues: paramValues,
		KindOptions: taskInfo.kindOptions,
	})
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}

	cmd := exec.CommandContext(ctx, cmds[0], cmds[1:]...)
	logger.Debug("Running %s", logger.Bold(strings.Join(cmd.Args, " ")))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "stdout")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "stderr")
	}

	// Load environment variables from .env files:
	env, err := getDevEnv(r, entrypoint)
	if err != nil {
		return err
	}
	// cmd.Env defaults to os.Environ _only if empty_. Since we add
	// to it, we need to also set it to os.Environ.
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	cmd.Env = append(cmd.Env, "AP_TEMPORAL_HOST=localhost:7333")
	cmd.Env = append(cmd.Env, fmt.Sprintf("AP_TASK_QUEUE=%s", taskQueue))
	cmd.Env = append(cmd.Env, fmt.Sprintf("AP_NAMESPACE=%s", namespace))

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "starting")
	}

	// mu guards o and chunks
	var mu sync.Mutex
	var o ojson.Value
	chunks := make(map[string]*strings.Builder)

	logParser := func(r io.Reader) error {
		scanner := bufiox.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			scanForErrors(cfg.root, line)
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

			logger.Log("[%s] %s", logger.Gray("log"), line)
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
		return err
	}

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "waiting")
	}

	print.Outputs(api.Outputs(o))

	analytics.Track(cfg.root, "Run Executed Locally", map[string]interface{}{
		"kind":         taskInfo.kind,
		"task_slug":    taskInfo.slug,
		"task_name":    taskInfo.name,
		"env_slug":     cfg.envSlug,
		"num_params":   len(paramValues),
		"num_env_vars": len(cmd.Env),
	}, analytics.TrackOpts{
		SkipSlack: true,
	})

	return nil
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
