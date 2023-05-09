package createdb

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/cli/auth/login"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/spf13/cobra"
)

type Config struct {
	Root *cli.Config
	Name string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = Config{Root: c}

	cmd := &cobra.Command{
		Use:   "create-db",
		Short: "Create demo DB",
		Long:  "Creates a demo SQL DB resource if it doesn't already exist",
		Example: heredoc.Doc(`
			airplane demo create-db
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd.Root().Context(), cfg)
		},
	}

	cmd.Flags().StringVar(&cfg.Name, "name", "[Demo DB]", "The name of the demo DB resource to create")

	return cmd
}

func Run(ctx context.Context, cfg Config) error {
	resourceID, err := cfg.Root.Client.CreateDemoDB(ctx, cfg.Name)
	if err != nil {
		return err
	}
	logger.Log(fmt.Sprintf("Resource %s has been created:", cfg.Name))
	logger.Log(fmt.Sprintf("%s/settings/resources/%s", cfg.Root.Client.AppURL().String(), resourceID))
	logger.Debug("Demo DB has resource ID %s", resourceID)
	return nil
}
