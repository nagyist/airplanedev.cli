package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	root          *cli.Config
	dir           string
	envSlug       string
	port          int
	devConfigPath string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = config{root: c}
	var err error

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Starts the local dev server",
		Long:  "Starts the local dev server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				cfg.dir = args[0]
			} else {
				if cfg.dir, err = os.Getwd(); err != nil {
					return errors.Wrap(err, "error determining current working directory")
				}
			}

			return run(cmd.Root().Context(), cfg)
		},
		// TODO: Support multiple dev roots
		Args: cobra.MaximumNArgs(1),
	}

	cmd.Flags().IntVar(&cfg.port, "port", server.DefaultPort, "The port to start the local airplane api server on - defaults to 7190.")
	cmd.Flags().StringVar(&cfg.devConfigPath, "config-path", "", "The path to the dev config file to load into the local dev server.")

	return cmd
}

func run(ctx context.Context, cfg config) error {
	// The API client is set in the root command, and defaults to api.airplane.dev as the host for deploys, etc. For
	// local dev, we send requests to a locally running api server, and so we override the host here.
	cfg.root.Client.Host = fmt.Sprintf("127.0.0.1:%d", cfg.port)

	localExecutor := &dev.LocalExecutor{}
	var devConfig conf.DevConfig
	var err error

	if cfg.devConfigPath != "" {
		devConfig, err = conf.ReadDevConfig(cfg.devConfigPath)
		if err != nil {
			return errors.Wrap(err, "loading in dev config file")
		}
	}

	apiServer, err := server.Start(server.Options{
		CLI:       cfg.root,
		EnvSlug:   cfg.envSlug,
		Executor:  localExecutor,
		Port:      cfg.port,
		DevConfig: devConfig,
	})
	if err != nil {
		return errors.Wrap(err, "starting local dev server")
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	logger.Log("Discovering tasks and views...")

	// Discover local tasks and views in the directory of the file.
	d := &discover.Discoverer{
		TaskDiscoverers: []discover.TaskDiscoverer{
			&discover.DefnDiscoverer{
				Client: cfg.root.Client,
			},
		},
		EnvSlug: cfg.envSlug,
		Client:  cfg.root.Client,
	}

	taskConfigs, viewConfigs, err := d.Discover(ctx, cfg.dir)
	if err != nil {
		return errors.Wrap(err, "discovering task configs")
	}

	// Print out discovered views and tasks to the user
	taskNoun := "tasks"
	if len(taskConfigs) == 1 {
		taskNoun = "task"
	}
	viewNoun := "views"
	if len(viewConfigs) == 1 {
		viewNoun = "view"
	}

	logger.Log("Found %d %s and %d %s:", len(taskConfigs), taskNoun, len(viewConfigs), viewNoun)
	for _, task := range taskConfigs {
		logger.Log("- %s", task.Def.GetName())
	}

	for _, view := range viewConfigs {
		logger.Log("- %s", view.Def.Name)
	}

	// Register discovered tasks with local dev server
	apiServer.RegisterTasksAndViews(taskConfigs, viewConfigs)

	logger.Log("")
	logger.Log("Visit https://app.airplane.dev/editor?host=http://localhost:%d for a development UI.", cfg.port)
	logger.Log("[Ctrl+C] to shutdown the local dev server.")

	// Wait for termination signal (e.g. Ctrl+C)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Stop(ctx); err != nil {
		return errors.Wrap(err, "stopping api server")
	}

	return nil
}
