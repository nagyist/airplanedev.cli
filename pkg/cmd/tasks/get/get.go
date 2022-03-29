package get

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/print"
	"github.com/airplanedev/lib/pkg/api"
	"github.com/spf13/cobra"
)

type config struct {
	root *cli.Config

	slug    string
	envSlug string
}

// New returns a new get command.
func New(c *cli.Config) *cobra.Command {
	cfg := config{
		root: c,
	}

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get information about a task",
		Example: heredoc.Doc(`
			airplane tasks get my_task
			airplane tasks get my_task -o yaml
			airplane tasks get my_task -o json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.slug = args[0]
			return run(cmd.Root().Context(), cfg)
		},
	}

	// Unhide this flag once we release environments.
	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment to query. Defaults to your team's default environment.")

	return cmd
}

// Run runs the get command.
func run(ctx context.Context, cfg config) error {
	var client = cfg.root.Client

	task, err := client.GetTask(ctx, api.GetTaskRequest{
		Slug:    cfg.slug,
		EnvSlug: cfg.envSlug,
	})
	if err != nil {
		return err
	}

	print.Task(task)
	return nil
}
