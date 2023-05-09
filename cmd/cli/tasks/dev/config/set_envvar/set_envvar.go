package set_envvar

import (
	"context"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	devCLI *cli.DevCLI
	key    string
	value  string
}

func New(c *cli.DevCLI) *cobra.Command {
	var cfg = config{devCLI: c}
	cmd := &cobra.Command{
		Use:   "set-envvar",
		Short: "Sets an environment variable in the dev config file",
		Example: heredoc.Doc(`
			airplane dev config set-envvar <key> <value>
			airplane dev config set-envvar API_KEY test
			airplane dev config set-envvar <key>=<value>
			airplane dev config set-envvar API_KEY=test
		`),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 {
				cfg.key = args[0]
				cfg.value = args[1]
			} else {
				kvPair := strings.Split(args[0], "=")
				if len(kvPair) != 2 {
					return errors.New("key and value must be separate arguments or of the form <key>=<value>")
				}
				cfg.key = kvPair[0]
				cfg.value = kvPair[1]
			}

			return run(cmd.Root().Context(), cfg)
		},
	}

	return cmd
}

func run(ctx context.Context, cfg config) error {
	return cfg.devCLI.DevConfig.SetEnvVar(cfg.key, cfg.value)
}
