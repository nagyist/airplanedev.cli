package tasks

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/cmd/airplane/root/deploy"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev"
	"github.com/airplanedev/cli/cmd/airplane/tasks/execute"
	"github.com/airplanedev/cli/cmd/airplane/tasks/get"
	"github.com/airplanedev/cli/cmd/airplane/tasks/initcmd"
	"github.com/airplanedev/cli/cmd/airplane/tasks/list"
	"github.com/airplanedev/cli/cmd/airplane/tasks/open"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/spf13/cobra"
)

// New returns a new cobra command.
func New(c *cli.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tasks",
		Short:   "Manage tasks",
		Long:    "Manage tasks",
		Aliases: []string{"task"},
		Example: heredoc.Doc(`
			airplane tasks init
			airplane tasks deploy my_task.airplane.ts
			airplane tasks get my_task
			airplane tasks execute my_task
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
	}

	cmd.AddCommand(deploy.New(c))
	cmd.AddCommand(list.New(c))
	cmd.AddCommand(dev.New(c))
	cmd.AddCommand(execute.New(c))
	cmd.AddCommand(get.New(c))
	cmd.AddCommand(initcmd.New(c))
	cmd.AddCommand(open.New(c))

	return cmd
}
