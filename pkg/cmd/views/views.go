package views

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/cmd/auth/login"
	"github.com/airplanedev/cli/pkg/cmd/tasks/deploy"
	"github.com/airplanedev/cli/pkg/cmd/views/dev"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/spf13/cobra"
)

// New returns a new cobra command.
func New(c *cli.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "views",
		Short:   "Manage views",
		Long:    "Manage views",
		Aliases: []string{"view"},
		Example: heredoc.Doc(`
			airplane views dev
			airplane views deploy
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		Hidden: true,
	}

	cmd.AddCommand(deploy.New(c))
	cmd.AddCommand(dev.New(c))

	return cmd
}
