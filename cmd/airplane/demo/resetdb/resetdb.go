package resetdb

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/spf13/cobra"
)

type Config struct {
	Root *cli.Config
}

func New(c *cli.Config) *cobra.Command {
	var cfg = Config{Root: c}

	cmd := &cobra.Command{
		Use:   "reset-db",
		Short: "Reset demo DB",
		Long:  "Resets the SQL DB resource [Demo DB] to its original state",
		Example: heredoc.Doc(`
			airplane demo reset-db
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd.Root().Context(), cfg)
		},
	}
	return cmd
}

func Run(ctx context.Context, cfg Config) error {
	resourceID, err := cfg.Root.Client.ResetDemoDB(ctx)
	if err != nil {
		return err
	}
	logger.Log("Resource [Demo DB] has been reset:")
	logger.Log(fmt.Sprintf("%s/settings/resources/%s", cfg.Root.Client.AppURL().String(), resourceID))
	logger.Debug("Demo DB has resource ID %s", resourceID)
	return nil
}
