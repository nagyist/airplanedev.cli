package demo

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/cli/auth/login"
	"github.com/airplanedev/cli/cmd/cli/demo/createdb"
	"github.com/airplanedev/cli/cmd/cli/demo/resetdb"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/spf13/cobra"
)

// New returns a new cobra command.
func New(c *cli.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Handles demo-related commands",
		Long:  "Handles demo-related commands",
		Example: heredoc.Doc(`
			airplane demo create-db
			airplane demo reset-db
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
	}

	cmd.AddCommand(createdb.New(c))
	cmd.AddCommand(resetdb.New(c))

	return cmd
}
