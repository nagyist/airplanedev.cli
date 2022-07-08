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
	viewsdev "github.com/airplanedev/cli/cmd/airplane/views/dev"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/params"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/utils"
	libBuild "github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	root      *cli.Config
	fileOrDir string
	args      []string
	envSlug   string
	// TODO: Include this in dev config file
	port int
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
	cmd.Flags().IntVar(&cfg.port, "port", server.DefaultPort, "The port to start the local airplane api server on - defaults to 7190.")
	return cmd
}

func run(ctx context.Context, cfg config) error {
	// The API client is set in the root command, and defaults to api.airplane.dev as the host for deploys, etc. For
	// local dev, we send requests to a locally running api server, and so we override the host here.
	cfg.root.Client.Host = fmt.Sprintf("127.0.0.1:%d", cfg.port)

	if !fsx.Exists(cfg.fileOrDir) {
		return errors.Errorf("Unable to open: %s", cfg.fileOrDir)
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

	// Start local api server for workflow tasks only
	localExecutor := &dev.LocalExecutor{}
	if taskInfo.runtime == libBuild.TaskRuntimeWorkflow {
		apiServer, err := server.Start(server.Options{
			CLI:      cfg.root,
			EnvSlug:  cfg.envSlug,
			Executor: localExecutor,
			Port:     cfg.port,
		})
		if err != nil {
			return errors.Wrap(err, "starting local dev api server")
		}

		defer func() {
			if err := apiServer.Stop(); err != nil {
				logger.Error("failed to stop local api server: %+v", err)
			}
		}()

		// Discover local tasks in the directory of the file.
		d := &discover.Discoverer{
			TaskDiscoverers: []discover.TaskDiscoverer{
				&discover.DefnDiscoverer{
					Client: cfg.root.Client,
				},
			},
			EnvSlug: cfg.envSlug,
			Client:  cfg.root.Client,
		}
		wd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, "error getting current working directory")
		}
		taskConfigs, _, err := d.Discover(ctx, filepath.Dir(filepath.Join(wd, cfg.fileOrDir)))
		if err != nil {
			return errors.Wrap(err, "discovering task configs")
		}

		// TODO: Allow users to re-register tasks once we move to a long-running local api server
		apiServer.RegisterTasks(taskConfigs)
	}

	paramValues, err := params.CLI(cfg.args, taskInfo.name, taskInfo.parameters)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}

	if err := localExecutor.Execute(ctx, dev.LocalRunConfig{
		Name:        taskInfo.name,
		Kind:        taskInfo.kind,
		KindOptions: taskInfo.kindOptions,
		ParamValues: paramValues,
		Port:        cfg.port,
		Root:        cfg.root,
		File:        cfg.fileOrDir,
		Slug:        taskInfo.slug,
		EnvSlug:     cfg.envSlug,
	}); err != nil {
		return errors.Wrap(err, "executing task")
	}

	return nil
}
