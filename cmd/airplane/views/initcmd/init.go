package initcmd

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/initcmd"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/spf13/cobra"
)

type config struct {
	root   *cli.Config
	dryRun bool
	name   string
	from   string
	cmd    *cobra.Command
}

func New(c *cli.Config) *cobra.Command {
	var cfg = GetConfig(c)

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Initialize a view definition",
		Example: heredoc.Doc("$ airplane views init"),
		// TODO: support passing in where to create the directory either as arg or flag
		Args: cobra.MaximumNArgs(0),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd.Root().Context(), cfg)
		},
	}

	cmd.Flags().BoolVarP(&cfg.dryRun, "dry-run", "", false, "True to run a dry run of this command.")
	if err := cmd.Flags().MarkHidden("dry-run"); err != nil {
		logger.Debug("error: %s", err)
	}

	cmd.Flags().StringVar(&cfg.from, "from", "", "Path to an existing github URL to initialize from")
	cfg.cmd = cmd

	return cmd
}

func GetConfig(c *cli.Config) config {
	return config{root: c}
}

func Run(ctx context.Context, cfg config) error {
	if cfg.from != "" {
		if err := initcmd.InitViewFromExample(ctx, initcmd.InitViewFromExampleRequest{
			Prompter:    cfg.root.Prompter,
			ExamplePath: cfg.from,
		}); err != nil {
			return err
		}
	} else {
		if err := promptForNewView(&cfg); err != nil {
			return err
		}
		if _, err := initcmd.InitView(ctx, initcmd.InitViewRequest{
			Prompter: cfg.root.Prompter,
			DryRun:   cfg.dryRun,
			Name:     cfg.name,
		}); err != nil {
			return err
		}
	}
	return nil
}

func promptForNewView(config *config) error {
	return config.root.Prompter.Input("What should this view be called?", &config.name)
}
