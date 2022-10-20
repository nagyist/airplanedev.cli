package dev

import (
	"context"
	_ "embed"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/views"
	"github.com/airplanedev/cli/pkg/views/viewdir"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type Config struct {
	Root      *cli.Config
	FileOrDir string
	Args      []string
	EnvSlug   string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = Config{Root: c}

	cmd := &cobra.Command{
		Use:   "dev [./path/to/directory]",
		Short: "Locally run a view",
		Long:  "Locally runs a view from the view's directory",
		Example: heredoc.Doc(`
			airplane views dev
			airplane views dev ./path/to/directory
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			// TODO: update the `dev` command to work w/out internet access
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				wd, err := os.Getwd()
				if err != nil {
					return errors.Wrap(err, "error determining current working directory")

				}
				cfg.FileOrDir = wd
			} else {
				cfg.FileOrDir = args[0]
			}

			return Run(cmd.Root().Context(), cfg)
		},
		Deprecated: "please use `airplane dev` instead.",
	}

	cmd.Flags().StringVar(&cfg.EnvSlug, "env", "", "The slug of the environment to run the view against. Defaults to your team's default environment.")

	return cmd
}

func Run(ctx context.Context, cfg Config) error {
	if !fsx.Exists(cfg.FileOrDir) {
		return errors.Errorf("Unable to open: %s", cfg.FileOrDir)
	}

	return StartView(ctx, cfg)
}

// StartView starts a view development server.
func StartView(ctx context.Context, cfg Config) error {
	rootDir, err := viewdir.FindRoot(cfg.FileOrDir)
	if err != nil {
		return err
	}
	vd, err := viewdir.NewViewDirectory(ctx, cfg.Root.Client, rootDir, cfg.FileOrDir, cfg.EnvSlug)
	if err != nil {
		return err
	}

	cmd, _, closer, err := views.Dev(ctx, &vd, views.ViteOpts{
		Client:  cfg.Root.Client,
		EnvSlug: cfg.EnvSlug,
		TTY:     true,
	})
	if err != nil {
		return err
	}
	defer closer.Close()
	return cmd.Wait()
}
