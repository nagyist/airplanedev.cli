package initcmd

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/spf13/cobra"
)

type config struct {
	client  *api.Client
	envSlug string
	name    string
	viewDir string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = config{client: c.Client}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a view definition",
		Example: heredoc.Doc(`
			$ airplane views init
		`),
		// TODO: support passing in where to create the directory either as arg or flag
		Args: cobra.MaximumNArgs(0),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Root().Context(), cfg)
		},
	}
	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment to query. Defaults to your team's default environment.")

	return cmd
}

func run(ctx context.Context, cfg config) error {
	if err := promptForNewView(&cfg); err != nil {
		return err
	}

	if err := createViewScaffolding(&cfg); err != nil {
		return err
	}
	return nil
}

func promptForNewView(config *config) error {
	if err := survey.AskOne(
		&survey.Input{
			Message: "What should this view be called?",
		},
		&config.name,
	); err != nil {
		return err
	}
	return nil
}

func createViewScaffolding(cfg *config) error {
	// Default to creating folder with the slug
	directory := utils.MakeSlug(cfg.name)
	if fsx.Exists(directory) {
		question := fmt.Sprintf("Directory %s already exists. Do you want to remove its existing files and continue creating view?", directory)

		if ok, err := utils.Confirm(question); err != nil {
			return err
		} else if !ok {
			logger.Log("❌ airplane views init canceled")
			return nil
		}
		os.RemoveAll(directory)
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	cfg.viewDir = directory
	// TODO: Add the views scaffolding files to directory
	if err := createViewDefinition(*cfg); err != nil {
		return err
	}
	return nil
}

func createViewDefinition(cfg config) error {
	if cfg.name == "" {
		return errors.New("missing new view name")
	}

	def := definitions.ViewDefinition{
		Name:       cfg.name,
		Slug:       utils.MakeSlug(cfg.name),
		Entrypoint: ".",
	}

	defnFilename := fmt.Sprintf("%s/%s.view.yaml", cfg.viewDir, def.Slug)

	buf, err := def.GenerateCommentedFile()
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(defnFilename, buf, 0644); err != nil {
		return err
	}
	logger.Step("Created view definition at %s", defnFilename)
	return nil
}
