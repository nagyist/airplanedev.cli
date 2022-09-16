package config

import (
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev/config/create_demo_db"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev/config/delete_config"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev/config/delete_resource"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev/config/set_config"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev/config/set_resource"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/utils"
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
			airplane dev config set-env API_KEY=test
			airplane dev config delete-env API_KEY
			airplane dev config set-resource --kind postgres db
			airplane dev config delete-resource db
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			if cfg.Filepath == "" {
				wd, err := os.Getwd()
				if err != nil {
					return errors.Wrap(err, "error determining current working directory")
				}

				defaultPath := filepath.Join(wd, conf.DefaultDevConfigFileName)
				if _, err := os.Stat(defaultPath); err == nil {
					cfg.Filepath = defaultPath
				} else {
					path, err := conf.PromptDevConfigFileCreation(conf.DefaultDevConfigFileName)
					if err != nil {
						return err
					} else if path == "" {
						return errors.New("Dev config file must exist")
					}
					cfg.Filepath = path
				}
			}

			var err error
			cfg.DevConfig, err = conf.ReadDevConfig(cfg.Filepath)
			if err != nil {
				return err
			}

			return nil
		}),
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
	cmd.AddCommand(create_demo_db.New(cfg))

	return cmd
}
