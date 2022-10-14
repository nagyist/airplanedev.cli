package config

import (
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev/config/delete_config"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev/config/delete_resource"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev/config/set_config"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev/config/set_resource"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// New returns a new cobra command.
func New(c *cli.Config) *cobra.Command {
	var cfg = &cli.DevCLI{
		Config: c,
	}

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage dev config",
		Long:  "Manage dev config",
		Example: heredoc.Doc(`
			airplane dev config set-configvar API_KEY=test
			airplane dev config delete-configvar API_KEY
			airplane dev config set-resource --kind postgres db
			airplane dev config delete-resource db
		`),
		// The subcommands of `airplane dev config` don't have PersistentPreRunE functions, and so they automatically
		// inherit the parent run's PersistentPreRunE. If we wrapped this in utils.WithParentPersistentPreRunE, this
		// would cause the dev config file to get loaded twice (once by the subcommand, and once by the parent command).
		// This command doesn't need any of its parent's PersistentPreRunE checks (e.g. checking login), and so we omit
		// the wrapper here.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cfg.Filepath == "" {
				wd, err := os.Getwd()
				if err != nil {
					return errors.Wrap(err, "error determining current working directory")
				}
				cfg.Filepath = filepath.Join(wd, conf.DefaultDevConfigFileName)
			}

			var err error
			cfg.DevConfig, err = conf.NewDevConfigFile(cfg.Filepath)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(
		&cfg.Filepath,
		"config-path",
		"c",
		"",
		"Path to airplane dev config file",
	)

	cmd.AddCommand(set_config.New(cfg))
	cmd.AddCommand(delete_config.New(cfg))

	cmd.AddCommand(set_resource.New(cfg))
	cmd.AddCommand(delete_resource.New(cfg))

	return cmd
}
