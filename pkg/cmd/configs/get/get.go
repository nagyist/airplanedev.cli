package get

import (
	"context"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/configs"
	"github.com/airplanedev/cli/pkg/print"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	root *cli.Config

	name    string
	envSlug string
}

// New returns a new get command.
func New(c *cli.Config) *cobra.Command {
	cfg := config{
		root: c,
	}
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get a config variable's value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.name = args[0]
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

	nt, err := configs.ParseName(cfg.name)
	if err != nil {
		return errors.Errorf("invalid config name: %s - expected my_config or my_config:tag", cfg.name)
	}
	resp, err := client.GetConfig(ctx, api.GetConfigRequest{
		Name:       nt.Name,
		Tag:        nt.Tag,
		ShowSecret: false,
		EnvSlug:    cfg.envSlug,
	})
	if err != nil {
		return errors.Wrap(err, "get config")
	}

	return print.Config(resp.Config)
}
