package list

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/cli/print"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	root *cli.Config

	envSlug string
}

// New returns a new list command.
func New(c *cli.Config) *cobra.Command {
	cfg := config{
		root: c,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all tasks",
		Example: heredoc.Doc(`
			airplane tasks list
			airplane tasks list -o json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Root().Context(), cfg)
		},
	}

	// Unhide this flag once we release environments.
	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment to query. Defaults to your team's default environment.")

	return cmd
}

// Run runs the list command.
func run(ctx context.Context, cfg config) error {
	var client = cfg.root.Client

	res, err := client.ListTasks(ctx, cfg.envSlug)
	if err != nil {
		return errors.Wrap(err, "list tasks")
	}

	if len(res.Tasks) == 0 {
		logger.Log(heredoc.Doc(`
			There are no tasks yet.

			Check out the getting started guides: https://docs.airplane.dev/getting-started/tasks
		`))
		return nil
	}

	print.Tasks(res.Tasks)
	return nil
}
