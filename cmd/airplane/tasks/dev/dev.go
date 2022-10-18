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
	viewsdev "github.com/airplanedev/cli/cmd/airplane/views/dev"
	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/configs"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/params"
	"github.com/airplanedev/cli/pkg/resource"
	"github.com/airplanedev/cli/pkg/server"
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
	// TODO: Remove these fields once launching the dev server is the default behavior of airplane dev
	fileOrDir string
	args      []string
	// If there are multiple tasks a, b in file f (config as code), specifying airplane
	// dev f::a would set fileOrDir to f and entrypointFunc to a.
	entrypointFunc string

	// Airplane dev server-related fields
	studio         bool
	useFallbackEnv bool
	watch          bool
}

func New(c *cli.Config) *cobra.Command {
	var cfg = taskDevConfig{root: c}

	cmd := &cobra.Command{
		Use:   "dev ./path/to/file",
		Short: "Locally run a task",
		Long:  "Locally runs a task, optionally with specific parameters.",
		Example: heredoc.Doc(`
			airplane dev ./task.js [-- <parameters...>]
			airplane dev ./task.ts::<exportName> [-- <parameters...>] (for multiple tasks in one file)
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			// TODO: update the `dev` command to work w/out internet access
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
					return errors.New("Multiple dev server roots detected, please supply only one directory to discover tasks and views")
				} else {
					// Use absolute path to dev root to allow the local dev server to more easily calculate relative paths.
					cfg.fileOrDir = args[0]
					if cfg.fileOrDir, err = filepath.Abs(cfg.fileOrDir); err != nil {
						return errors.Wrap(err, "getting absolute path of studio working directory")
					}
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

					// Recursively search for dev config file, starting from the dev dir.
					devConfigDir, ok := fsx.Find(devDir, conf.DefaultDevConfigFileName)
					if !ok {
						// If a dev config file is not found, set the dev config dir to the dev root and prompt for creation
						// of the file below.
						devConfigDir = devDir
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

			cfg.devConfig, err = conf.NewDevConfigFile(cfg.devConfigPath)
			if err != nil {
				return errors.Wrap(err, "loading dev config file")
			}

			return run(cmd.Root().Context(), cfg)
		},
	}

	cmd.AddCommand(config.New(c))

	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment to query. Defaults to your team's default environment.")
	cmd.Flags().IntVar(&cfg.port, "port", server.DefaultPort, "The port to start the local airplane api server on - defaults to 4000.")
	cmd.Flags().StringVar(&cfg.devConfigPath, "config-path", "", "The path to the dev config file to load into the local dev server.")
	// TODO: Make opening the studio the default behavior.
	cmd.Flags().BoolVar(&cfg.studio, "studio", false, "Run the local airplane studio")
	cmd.Flags().BoolVar(&cfg.studio, "editor", false, "Run the local airplane studio (use --studio instead)")
	cmd.Flags().BoolVar(&cfg.watch, "watch", false, "Watch for changes and apply updates to tasks, views, and workflows automatically.")

	if err := cmd.Flags().MarkHidden("editor"); err != nil {
		logger.Debug("error: %s", err)
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

	fileInfo, err := os.Stat(cfg.fileOrDir)
	if err != nil {
		return errors.Wrapf(err, "describing %s", cfg.fileOrDir)
	}

	if fileInfo.IsDir() {
		if viewsdev.IsView(cfg.fileOrDir) == nil {
			return viewsdev.Run(ctx, viewsdev.Config{
				Root:      cfg.root,
				FileOrDir: cfg.fileOrDir,
				Args:      cfg.args,
				EnvSlug:   cfg.envSlug,
			})
		}
		return errors.Errorf("%s is a directory", cfg.fileOrDir)
	}

	localExecutor := &dev.LocalExecutor{}
	localClient := &api.Client{
		Host:   fmt.Sprintf("127.0.0.1:%d", cfg.port),
		Token:  cfg.root.Client.Token,
		Source: cfg.root.Client.Source,
		APIKey: cfg.root.Client.APIKey,
		TeamID: cfg.root.Client.TeamID,
	}

	apiServer, err := server.Start(server.Options{
		LocalClient:  localClient,
		RemoteClient: cfg.root.Client,
		Executor:     localExecutor,
		Port:         cfg.port,
		DevConfig:    cfg.devConfig,
	})
	if err != nil {
		return errors.Wrap(err, "starting local dev api server")
	}

	defer func() {
		if err := apiServer.Stop(context.Background()); err != nil {
			l.Error("failed to stop local api server: %+v", err)
		}
	}()

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
	resources, err := resource.GenerateAliasToResourceMap(
		ctx,
		nil,
		taskConfig.Def.GetResourceAttachments(),
		cfg.devConfig.Resources,
	)
	if err != nil {
		return errors.Wrap(err, "generating alias to resource map")
	}

	kind, kindOptions, err := dev.GetKindAndOptions(taskConfig)
	if err != nil {
		return err
	}

	envVars, err := dev.MaterializeEnvVars(taskConfig, cfg.devConfig)
	if err != nil {
		return err
	}
	attachedConfigs, err := taskConfig.Def.GetConfigAttachments()
	if err != nil {
		return errors.Wrap(err, "getting attached configs")
	}
	configVars := configs.MaterializeConfigs(attachedConfigs, cfg.devConfig.ConfigVars)

	localRunConfig := dev.LocalRunConfig{
		ID:           dev.GenerateRunID(),
		Name:         taskConfig.Def.GetName(),
		Kind:         kind,
		KindOptions:  kindOptions,
		ParamValues:  paramValues,
		LocalClient:  localClient,
		RemoteClient: cfg.root.Client,
		File:         cfg.fileOrDir,
		Slug:         taskConfig.Def.GetSlug(),
		EnvVars:      envVars,
		Resources:    resources,
		ConfigVars:   configVars,
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
		"num_config_vars": len(envVars),
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
