package agents

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/cli/agents/ecslogs"
	"github.com/airplanedev/cli/cmd/cli/auth/login"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/spf13/cobra"
)

// New returns a new cobra command.
func New(c *cli.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Debug self-hosted agents",
		Long:  "Debug self-hosted agents",
		Example: heredoc.Doc(`
			airplane agents ecslogs --cluster-name=my-airplane-ecs-cluster
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
	}

	cmd.AddCommand(ecslogs.New(c))

	return cmd
}
