package create_demo_db

import (
	"context"
	"encoding/json"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	devCLI *cli.DevCLI
}

func New(c *cli.DevCLI) *cobra.Command {
	var cfg = config{devCLI: c}
	cmd := &cobra.Command{
		Use:   "create-demo-db",
		Short: "Create demo DB resource in the dev config file",
		Example: heredoc.Doc(`
			airplane dev config create-demo-db
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Root().Context(), cfg)
		},
	}

	return cmd
}

func run(ctx context.Context, cfg config) error {
	demoDBResource, err := cfg.devCLI.Client.GetDemoDBResource(ctx)
	if err != nil {
		return errors.Wrap(err, "getting demo db resource")
	}
	if demoDBResource.KindConfig.Postgres == nil {
		return errors.Wrap(err, "expected demo DB to be postgres resource")
	}
	postgresConfig := demoDBResource.KindConfig.Postgres

	slug := "demo_db"
	serializedResource := map[string]interface{}{}
	serializedResource["kind"] = kinds.ResourceKindPostgres
	serializedResource["slug"] = slug
	serializedResource["name"] = demoDBResource.Name
	serializedResource["host"] = postgresConfig.Host
	serializedResource["port"] = postgresConfig.Port
	serializedResource["database"] = postgresConfig.Database
	serializedResource["username"] = postgresConfig.Username
	serializedResource["password"] = postgresConfig.Password
	if postgresConfig.DisableSSL {
		serializedResource["ssl"] = "disable"
	} else {
		serializedResource["ssl"] = "require"
	}

	resource, err := resources.GetResource(kinds.ResourceKindPostgres, serializedResource)
	if err != nil {
		return errors.Wrap(err, "unserializing demo db resource")
	}

	devConfig := cfg.devCLI.DevConfig
	if err := devConfig.SetResource(slug, resource); err != nil {
		return errors.Wrap(err, "setting resource in dev config file")
	}
	devConfig.RawResources = append(devConfig.RawResources, serializedResource)

	if err := conf.WriteDevConfig(devConfig); err != nil {
		return err
	}

	encodedResource, err := json.MarshalIndent(resource, "", "  ")
	if err != nil {
		return err
	}
	logger.Log(string(encodedResource))
	return nil
}
