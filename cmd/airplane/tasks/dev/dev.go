package dev

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev/config"
	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/params"
	"github.com/airplanedev/cli/pkg/resources"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type taskDevConfig struct {
	root          *cli.Config
	port          int
	devConfigPath string
	devConfig     *conf.DevConfig
	envSlug       string

	// Short-lived dev command fields
	// TODO: Remove these fields once we remove legacy airplane dev behavior
	fileOrDir string
	args      []string
	// If there are multiple tasks a, b in file f (config as code), specifying airplane
	// dev f::a would set fileOrDir to f and entrypointFunc to a.
	entrypointFunc string

	// Airplane dev server-related fields
	studio           bool
	useFallbackEnv   bool
	disableWatchMode bool
	sandbox          bool
	tunnel           bool
	serverHost       string
	namespace        string
	key              string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = taskDevConfig{root: c}

	cmd := &cobra.Command{
		Use:   "dev ./path/to/file",
		Short: "Locally run a task",
		Long:  "Locally runs a task, optionally with specific parameters.",
		Example: heredoc.Doc(`
			airplane dev (develop all tasks and views in the current directory)
			airplane dev ./airplane_apps/ (developing all tasks and views in ./airplane_apps/)
			airplane dev ./my.task.yaml (developing a single task)
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return errors.Wrap(err, "error determining current working directory")
			}
			if cfg.studio {
				// TODO: Support multiple dev server roots
				if len(args) == 0 {
					cfg.fileOrDir = wd
				} else if len(args) > 1 {
					return errors.New("detected multiple arguments to `airplane dev`. Please pass in at most one directory to discover tasks and views")
				} else {
					cfg.fileOrDir = args[0]
				}

				if cfg.devConfigPath == "" {
					var devDir string
					// Calling filepath.Dir on a directory path that doesn't end in '/' returns that directory's parent -
					// we want to return the directory itself in that case, and so we check if cfg.fileOrDir is a directory.
					if info, err := os.Stat(cfg.fileOrDir); err != nil {
						return err
					} else if info.IsDir() {
						devDir = cfg.fileOrDir
					} else {
						devDir = filepath.Dir(cfg.fileOrDir)
					}

					absDevDir, err := filepath.Abs(devDir)
					if err != nil {
						return errors.Wrap(err, "getting absolute path of dev root")
					}

					// Recursively search for dev config file, starting from the dev dir.
					devConfigDir, ok := fsx.Find(absDevDir, conf.DefaultDevConfigFileName)
					if !ok {
						// If a dev config file is not found, set the dev config dir to the dev root and auto-create the
						// file whenever it needs to be written to.
						devConfigDir = absDevDir
					}
					cfg.devConfigPath = filepath.Join(devConfigDir, conf.DefaultDevConfigFileName)
				} else {
					cfg.devConfigPath, err = filepath.Abs(cfg.devConfigPath)
					if err != nil {
						return errors.Wrap(err, "getting absolute path of dev config file")
					}
				}

				// Enable fallback environments only if `--env` is explicitly set.
				if cmd.Flags().Changed("env") {
					cfg.useFallbackEnv = true
				}
			} else if len(args) == 0 || strings.HasPrefix(args[0], "-") {
				cfg.fileOrDir = wd
				cfg.args = args
			} else {
				cfg.fileOrDir = args[0]
				cfg.args = args[1:]
			}

			fileAndFunction := strings.Split(cfg.fileOrDir, "::")
			if len(fileAndFunction) > 1 {
				cfg.fileOrDir = fileAndFunction[0]
				cfg.entrypointFunc = fileAndFunction[1]
			}

			cfg.devConfig, err = conf.LoadDevConfigFile(cfg.devConfigPath)
			if err != nil {
				return errors.Wrap(err, "loading dev config file")
			}

			return run(cmd.Root().Context(), cfg)
		},
	}

	cmd.AddCommand(config.New(c))

	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the fallback environment to query for remote resources and configs. If not set, does not fall back to a remote environment")
	cmd.Flags().IntVar(&cfg.port, "port", 0, "The port to start the local airplane api server on - defaults to a random open port.")
	cmd.Flags().StringVar(&cfg.devConfigPath, "config-path", "", "The path to the dev config file to load into the local dev server.")
	cmd.Flags().BoolVar(&cfg.studio, "studio", true, "Run the local Studio")
	cmd.Flags().BoolVar(&cfg.studio, "editor", true, "Run the local Studio")
	cmd.Flags().BoolVar(&cfg.disableWatchMode, "no-watch", false, "Disable watch mode. Changes require restarting the studio to take effect.")
	cmd.Flags().BoolVar(&cfg.sandbox, "sandbox", false, "Run the Studio in a sandbox context (i.e. non-interactive, remote)")
	cmd.Flags().BoolVar(&cfg.tunnel, "tunnel", false, "Run the Studio with an ngrok tunnel")
	cmd.Flags().StringVar(&cfg.serverHost, "server-host", "", "Set the host from which the Studio should be accessed")
	// Namespace and key allow the Studio to get the token for the appropriate sandbox machine.
	cmd.Flags().StringVar(&cfg.namespace, "namespace", "", "The namespace when running the Studio in sandbox mode.")
	cmd.Flags().StringVar(&cfg.key, "key", "", "The namespace-specific key when running the Studio in sandbox mode")
	if err := cmd.Flags().MarkHidden("sandbox"); err != nil {
		logger.Debug("marking --sandbox as hidden: %v", err)
	}
	if err := cmd.Flags().MarkHidden("tunnel"); err != nil {
		logger.Debug("marking --tunnel as hidden: %v", err)
	}
	if err := cmd.Flags().MarkHidden("server-host"); err != nil {
		logger.Debug("marking --server-host as hidden: %v", err)
	}
	if err := cmd.Flags().MarkHidden("namespace"); err != nil {
		logger.Debug("marking --namespace as hidden: %v", err)
	}
	if err := cmd.Flags().MarkHidden("key"); err != nil {
		logger.Debug("marking --key as hidden: %v", err)
	}
	if err := cmd.Flags().MarkDeprecated("editor", "launching the Studio is now the default behavior."); err != nil {
		logger.Debug("marking --editor as deprecated: %s", err)
	}

	return cmd
}

func run(ctx context.Context, cfg taskDevConfig) error {
	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{})
	if cfg.studio {
		return runLocalDevServer(ctx, cfg)
	}

	if !fsx.Exists(cfg.fileOrDir) {
		return errors.Errorf("Unable to open: %s", cfg.fileOrDir)
	}

	apiServer, port, err := server.Start(server.Options{
		Port: cfg.port,
	})
	if err != nil {
		return errors.Wrap(err, "starting local dev api server")
	}

	defer func() {
		if err := apiServer.Stop(context.Background()); err != nil {
			l.Error("failed to stop local api server: %+v", err)
		}
	}()

	localExecutor := dev.NewLocalExecutor(filepath.Dir(cfg.fileOrDir))
	localClient := api.NewClient(api.ClientOpts{
		Host:   fmt.Sprintf("127.0.0.1:%d", port),
		Token:  cfg.root.Client.Token,
		Source: cfg.root.Client.Source,
		APIKey: cfg.root.Client.APIKey,
		TeamID: cfg.root.Client.TeamID,
	})

	apiServer.RegisterState(&state.State{
		LocalClient:  &localClient,
		RemoteClient: cfg.root.Client,
		Executor:     localExecutor,
		DevConfig:    cfg.devConfig,
	})

	// Discover local tasks in the directory of the file.
	d := &discover.Discoverer{
		TaskDiscoverers: []discover.TaskDiscoverer{
			&discover.DefnDiscoverer{
				Client: localClient,
				Logger: l,
			},
			&discover.CodeTaskDiscoverer{
				Client: localClient,
				Logger: l,
			},
		},
		EnvSlug: cfg.envSlug,
		Client:  localClient,
	}
	taskConfigs, viewConfigs, err := d.Discover(ctx, filepath.Dir(cfg.fileOrDir))
	if err != nil {
		return errors.Wrap(err, "discovering task configs")
	}
	taskConfig, err := getLocalDevTaskConfig(taskConfigs, cfg)
	if err != nil {
		return err
	}

	if _, err := apiServer.RegisterTasksAndViews(ctx, server.DiscoverOpts{
		Tasks: taskConfigs,
		Views: viewConfigs,
	}); err != nil {
		return err
	}
	parameters, err := taskConfig.Def.GetParameters()
	if err != nil {
		return err
	}
	paramValues, err := params.CLI(cfg.args, taskConfig.Def.GetName(), parameters)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}

	resourceAttachments, err := taskConfig.Def.GetResourceAttachments()
	if err != nil {
		return err
	}
	aliasToResource, err := resources.GenerateAliasToResourceMap(
		ctx,
		nil,
		resourceAttachments,
		cfg.devConfig.Resources,
	)
	if err != nil {
		return errors.Wrap(err, "generating alias to resource map")
	}

	kind, kindOptions, err := dev.GetKindAndOptions(taskConfig)
	if err != nil {
		return err
	}

	taskEnv, err := taskConfig.Def.GetEnv()
	if err != nil {
		return err
	}
	configAttachments, err := taskConfig.Def.GetConfigAttachments()
	if err != nil {
		return errors.Wrap(err, "getting attached configs")
	}

	localRunConfig := dev.LocalRunConfig{
		ID:                dev.GenerateRunID(),
		Name:              taskConfig.Def.GetName(),
		Kind:              kind,
		KindOptions:       kindOptions,
		ParamValues:       paramValues,
		LocalClient:       &localClient,
		RemoteClient:      cfg.root.Client,
		UseFallbackEnv:    false,
		File:              cfg.fileOrDir,
		Slug:              taskConfig.Def.GetSlug(),
		AliasToResource:   aliasToResource,
		ConfigAttachments: configAttachments,
		ConfigVars:        cfg.devConfig.ConfigVars,
		EnvVars:           cfg.devConfig.EnvVars,
		TaskEnvVars:       taskEnv,
		PrintLogs:         true,
		StudioURL:         *cfg.root.Client.AppURL(),
	}
	_, err = localExecutor.Execute(ctx, localRunConfig)
	if err != nil {
		return errors.Wrap(err, "executing task")
	}

	analytics.Track(cfg.root.Client, "Run Executed Locally", map[string]interface{}{
		"kind":            kind,
		"task_slug":       taskConfig.Def.GetSlug(),
		"task_name":       taskConfig.Def.GetName(),
		"env_slug":        cfg.envSlug,
		"num_params":      len(paramValues),
		"num_config_vars": len(taskEnv),
	}, analytics.TrackOpts{
		SkipSlack: true,
	})

	return nil
}

func getLocalDevTaskConfig(taskConfigs []discover.TaskConfig, cfg taskDevConfig) (discover.TaskConfig, error) {
	absPath, err := filepath.Abs(cfg.fileOrDir)
	if err != nil {
		return discover.TaskConfig{}, errors.Wrap(err, "converting file to absolute")
	}
	var potentialTaskConfigs []discover.TaskConfig
	for _, taskConfig := range taskConfigs {
		if taskConfig.Def.GetDefnFilePath() == absPath || taskConfig.TaskEntrypoint == absPath {
			potentialTaskConfigs = append(potentialTaskConfigs, taskConfig)
		}
	}

	if len(potentialTaskConfigs) == 0 {
		return discover.TaskConfig{}, errors.New("unable to find any task in file")
	}

	if len(potentialTaskConfigs) == 1 && cfg.entrypointFunc == "" {
		return potentialTaskConfigs[0], nil
	}

	for _, taskConfig := range potentialTaskConfigs {
		buildConfig, err := taskConfig.Def.GetBuildConfig()
		if err != nil {
			return discover.TaskConfig{}, errors.Wrap(err, "getting build config")
		}
		entrypointFunc, _ := buildConfig["entrypointFunc"].(string)

		if cfg.entrypointFunc == "" && entrypointFunc == "default" {
			return taskConfig, nil
		} else if cfg.entrypointFunc == entrypointFunc {
			return taskConfig, nil
		}
	}

	return discover.TaskConfig{}, errors.New("unable to find specified task in file")
}
