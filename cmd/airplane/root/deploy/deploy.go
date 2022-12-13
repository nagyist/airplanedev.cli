package deploy

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/bundlediscover"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/spf13/cobra"
)

type Config struct {
	Root         *cli.Config
	Client       api.APIClient
	Paths        []string
	ChangedFiles utils.NewlineFileValue
	EnvSlug      string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = Config{
		Root:   c,
		Client: c.Client,
	}

	cmd := &cobra.Command{
		Use:   "deploybundle",
		Short: "Deploy tasks and views",
		Long:  "Deploy code from a local directory as an Airplane task or view.",
		Example: heredoc.Doc(`
			airplane deploy
			airplane deploy my_directory
			airplane tasks deploy ./my_task.ts
			airplane tasks deploy my_directory ./my_task1.ts
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				cfg.Paths = args
			} else {
				// Default to current directory.
				cfg.Paths = []string{"."}
			}
			return run(cmd.Root().Context(), cfg)
		},
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		Hidden: true,
	}

	cmd.Flags().Var(&cfg.ChangedFiles, "changed-files", "A file with a list of file paths that were changed, one path per line. Only tasks with changed files will be deployed")
	cmd.Flags().StringVar(&cfg.EnvSlug, "env", "", "The slug of the environment to query. Defaults to your team's default environment.")

	return cmd
}

func run(ctx context.Context, cfg Config) error {
	return Deploy(ctx, cfg)
}

func Deploy(ctx context.Context, cfg Config) error {
	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{WithLoader: true})
	defer l.StopLoader()

	d := &bundlediscover.Discoverer{
		TaskDiscoverers: []discover.TaskDiscoverer{
			&discover.ScriptDiscoverer{
				Client:  cfg.Client,
				Logger:  l,
				EnvSlug: cfg.EnvSlug,
			},
			&discover.DefnDiscoverer{
				Client: cfg.Client,
				Logger: l,
			},
			&discover.CodeTaskDiscoverer{
				Client: cfg.Client,
				Logger: l,
			},
		},
		ViewDiscoverers: []discover.ViewDiscoverer{
			&discover.ViewDefnDiscoverer{Client: cfg.Client, Logger: l},
			&discover.CodeViewDiscoverer{Client: cfg.Client, Logger: l},
		},
		Client:  cfg.Client,
		Logger:  l,
		EnvSlug: cfg.EnvSlug,
	}

	bundles, err := d.Discover(ctx, cfg.Paths...)
	if err != nil {
		return err
	}

	return NewDeployer(cfg, l, DeployerOpts{}).Deploy(ctx, bundles)
}
