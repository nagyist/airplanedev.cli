package generate

import (
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/cmd/auth/login"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func New(c *cli.Config) *cobra.Command {
	var cfg = config{root: c}

	cmd := &cobra.Command{
		Use:   "generate ./path/to/file",
		Short: "Generate code for Airplane tasks",
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
