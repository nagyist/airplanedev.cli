package logout

import (
	"context"

	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/cli/conf"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func New(c *cli.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out of Airplane",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Root().Context(), c)
		},
	}
	return cmd
}

func run(ctx context.Context, c *cli.Config) error {
	cfg, err := conf.ReadDefaultUserConfig()
	if !errors.Is(err, conf.ErrMissing) {
		if err != nil {
			return err
		}

		delete(cfg.Tokens, c.Client.Host())

		if err := conf.WriteDefaultUserConfig(cfg); err != nil {
			return err
		}
	}

	logger.Log("Logged out.")

	return nil
}