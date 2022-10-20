package initcmd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/node"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	client         *api.Client
	envSlug        string
	name           string
	viewDir        string
	slug           string
	from           string
	gettingStarted bool
}

const gettingStartedExample = "github.com/airplanedev/examples/views/getting_started"

func New(c *cli.Config) *cobra.Command {
	var cfg = GetConfig(c.Client)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a view definition",
		Example: heredoc.Doc(fmt.Sprintf(`
			$ airplane views init
			$ airplane views init --from %s
		`, gettingStartedExample)),
		// TODO: support passing in where to create the directory either as arg or flag
		Args: cobra.MaximumNArgs(0),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd.Root().Context(), cfg)
		},
	}
	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment to query. Defaults to your team's default environment.")
	cmd.Flags().StringVar(&cfg.from, "from", "", "Path to an existing github folder to initialize.")
	cmd.Flags().BoolVar(&cfg.gettingStarted, "getting-started", false, "True to generate starter tasks and views.")

	return cmd
}

func GetConfig(client *api.Client) config {
	return config{client: client}
}

func Run(ctx context.Context, cfg config) error {
	if cfg.gettingStarted {
		_, err := createDemoDB(ctx, cfg)
		if err != nil {
			return err
		}
		if err := utils.CopyFromGithubPath(gettingStartedExample); err != nil {
			return err
		}
		(&cfg).viewDir = filepath.Base(gettingStartedExample)
	} else if cfg.from != "" {
		if err := utils.CopyFromGithubPath(cfg.from); err != nil {
			return err
		}
		(&cfg).viewDir = filepath.Base(cfg.from)
	} else {
		if err := promptForNewView(&cfg); err != nil {
			return err
		}
		if err := createViewScaffolding(ctx, &cfg); err != nil {
			return err
		}
	}
	suggestNextSteps(cfg)
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

func createViewScaffolding(ctx context.Context, cfg *config) error {
	slug := utils.MakeSlug(cfg.name)
	cfg.slug = slug

	// Default to creating folder with the slug
	directory := slug
	if err := utils.CreateDirectory(directory); err != nil {
		return err
	}
	cfg.viewDir = directory

	// TODO: Add the views scaffolding files to directory
	if err := createViewDefinition(*cfg); err != nil {
		return err
	}
	if err := createEntrypoint(*cfg); err != nil {
		return err
	}
	if err := node.CreateViewTSConfig(); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "getting working directory")
	}
	packageJSONDir := cwd
	if fsx.Exists(filepath.Join(cwd, cfg.viewDir, "package.json")) {
		packageJSONDir = filepath.Join(cwd, cfg.viewDir)
	}
	if err := node.CreatePackageJSON(packageJSONDir, node.NodeDependencies{
		Dependencies:    []string{"@airplane/views", "react", "react-dom"},
		DevDependencies: []string{"@types/react", "@types/react-dom", "typescript"},
	}); err != nil {
		return err
	}

	if err := utils.CreateDefaultGitignoreFile(".gitignore"); err != nil {
		return err
	}
	return nil
}

func generateEntrypointPath(cfg config, inViewDir bool) string {
	if inViewDir {
		return fmt.Sprintf("%s.view.tsx", cfg.slug)
	} else {
		return fmt.Sprintf("%s/%s.view.tsx", cfg.viewDir, cfg.slug)
	}
}

func generateDefinitionFilePath(cfg config) string {
	return fmt.Sprintf("%s/%s.view.yaml", cfg.viewDir, cfg.slug)
}

func createDemoDB(ctx context.Context, cfg config) (string, error) {
	demoDBName := "[Demo DB]"
	return cfg.client.CreateDemoDB(ctx, demoDBName)
}

func createViewDefinition(cfg config) error {
	if cfg.name == "" {
		return errors.New("missing new view name")
	}

	def := definitions.ViewDefinition{
		Name:       cfg.name,
		Slug:       cfg.slug,
		Entrypoint: generateEntrypointPath(cfg, true),
	}

	defnFilename := generateDefinitionFilePath(cfg)

	buf, err := def.GenerateCommentedFile()
	if err != nil {
		return err
	}

	if err := os.WriteFile(defnFilename, buf, 0644); err != nil {
		return err
	}
	logger.Step("Created view definition at %s", defnFilename)
	return nil
}

//go:embed scaffolding/default.view.tsx
var defaultEntrypoint []byte

func createEntrypoint(cfg config) error {
	entrypointPath := generateEntrypointPath(cfg, false)

	if err := os.WriteFile(entrypointPath, defaultEntrypoint, 0644); err != nil {
		return errors.Wrap(err, "creating view entrypoint")
	}
	logger.Step("Created view entrypoint at %s", entrypointPath)
	return nil
}

func suggestNextSteps(cfg config) {
	if cfg.viewDir != "" && cfg.slug != "" {
		logger.Suggest("✅ To complete your view:", fmt.Sprintf("Write your view logic in %s", generateEntrypointPath(cfg, false)))
	}

	logger.Suggest(
		"⚡ To develop your view locally:",
		"airplane dev %s",
		cfg.viewDir,
	)

	var deployDir string
	if cfg.viewDir != "" {
		deployDir = cfg.viewDir
	} else {
		deployDir = "."
	}
	logger.Suggest(
		"🛫 To deploy your view to Airplane:",
		"airplane deploy %s",
		deployDir,
	)
}
