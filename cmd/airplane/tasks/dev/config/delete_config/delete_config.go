package delete_config

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	devCLI *cli.DevCLI
	key    string
}

func New(c *cli.DevCLI) *cobra.Command {
	var cfg = config{devCLI: c}
	cmd := &cobra.Command{
		Use:   "delete-configvar",
		Short: "Deletes a config variable from the dev config file",
		Example: heredoc.Doc(`
			airplane dev config delete-configvar <key>
			airplane dev config delete-configvar <key1> <key2> ...
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
	devConfig := cfg.devCLI.DevConfig
	if _, ok := devConfig.ConfigVars[cfg.key]; !ok {
		return errors.Errorf("Config variable `%s` not found in dev config file", cfg.key)
	}

	delete(devConfig.ConfigVars, cfg.key)
	if err := conf.WriteDevConfig(devConfig); err != nil {
		return err
	}

	logger.Log("Deleted config variable `%s` from dev config file.", cfg.key)
	return nil
}
