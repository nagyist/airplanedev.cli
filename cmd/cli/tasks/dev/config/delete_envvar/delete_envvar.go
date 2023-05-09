package delete_envvar

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/spf13/cobra"
)

type config struct {
	devCLI *cli.DevCLI
	key    string
}

func New(c *cli.DevCLI) *cobra.Command {
	var cfg = config{devCLI: c}
	cmd := &cobra.Command{
		Use:   "delete-envvar",
		Short: "Deletes an environment variable from the dev config file",
		Example: heredoc.Doc(`
			airplane dev config delete-envvar <key>
			airplane dev config delete-envvar <key1> <key2> ...
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.key = args[0]
			return run(cmd.Root().Context(), cfg)
		},
	}

	return cmd
}

// Run runs the open command.
func run(ctx context.Context, cfg config) error {
	return cfg.devCLI.DevConfig.DeleteEnvVar(cfg.key)
}
