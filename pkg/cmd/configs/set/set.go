package set

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/configs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	root *cli.Config

	name    string
	value   *string
	secret  bool
	envSlug string
}

// New returns a new set command.
func New(c *cli.Config) *cobra.Command {
	cfg := config{
		root: c,
	}
	cmd := &cobra.Command{
		Use:   "set [--secret] <name> [<value>]",
		Short: "Set a new or existing config variable",
		Example: heredoc.Doc(`
			# Pass in a value to the prompt
			$ airplane configs set --secret db/url
			Config value: my_value_here
			
			# Pass in a value by piping it in via stdin
			$ cat my_secret_value.txt | airplane configs set --secret secret_config

			# Recommended for non-secrets only - pass in a value via arguments
			$ airplane configs set nonsecret_config my_value
		`),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 {
				cfg.value = &args[1]
			}
			cfg.name = args[0]
			return run(cmd.Root().Context(), c, cfg)
		},
	}
	cmd.Flags().BoolVar(&cfg.secret, "secret", false, "Whether to set config var as a secret")
	// Unhide this flag once we release environments.
	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment to query. Defaults to your team's default environment.")
	return cmd
}

// Run runs the set command.
func run(ctx context.Context, c *cli.Config, cfg config) error {
	var client = c.Client

	nt, err := configs.ParseName(cfg.name)
	if err != nil {
		return errors.Errorf("invalid config name: %s - expected my_config or my_config:tag", cfg.name)
	}

	var value string
	if cfg.value != nil {
		value = *cfg.value
	} else {
		var err error
		value, err = configs.ReadValue(cfg.secret)
		if err != nil {
			return err
		}
	}
	return configs.SetConfig(ctx, client, configs.SetConfigRequest{
		NameTag: nt,
		Value:   value,
		Secret:  cfg.secret,
		EnvSlug: cfg.envSlug,
	})
}
