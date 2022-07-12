package initcmd

import (
	"context"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
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
	if err := createEntrypoint(*cfg); err != nil {
		return err
	}
	if err := createPackageJSON(*cfg); err != nil {
		return err
	}
	return nil
}

func generateEntrypointPath(cfg config, inViewDir bool) string {
	slug := utils.MakeSlug(cfg.name)
	if inViewDir {
		return fmt.Sprintf("%s.view.tsx", slug)
	} else {
		return fmt.Sprintf("%s/%s.view.tsx", cfg.viewDir, slug)
	}
}

func createViewDefinition(cfg config) error {
	if cfg.name == "" {
		return errors.New("missing new view name")
	}

	def := definitions.ViewDefinition{
		Name:       cfg.name,
		Slug:       utils.MakeSlug(cfg.name),
		Entrypoint: generateEntrypointPath(cfg, true),
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

//go:embed scaffolding/default.view.tsx
var defaultEntrypoint []byte

func createEntrypoint(cfg config) error {
	entrypointPath := generateEntrypointPath(cfg, false)

	if err := ioutil.WriteFile(entrypointPath, defaultEntrypoint, 0644); err != nil {
		return errors.Wrap(err, "creating view entrypoint")
	}
	logger.Step("Created view entrypoint at %s", entrypointPath)
	return nil
}

// createPackageJSON ensures there is a package.json in the cwd or a parent directory with the views dependencies installed.
// If package.json exists in cwd, use it.
// If package.json exists in parent directory, ask user if they want to use that or create a new one.
// If package.json doesn't exist, create a new one.
// TODO: support entrypoint override
func createPackageJSON(cfg config) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	absEntrypointPath, err := filepath.Abs(filepath.Join(cwd, generateEntrypointPath(cfg, false)))
	if err != nil {
		return err
	}
	// Check if there's a package.json in the current or parent directory of entrypoint
	packageJSONDirPath, ok := fsx.Find(absEntrypointPath, "package.json")

	if ok {
		if packageJSONDirPath == cwd {
			// TODO: check if @airplane/views already is installed and if so, don't install again
			return yarnAddViewsPackage(packageJSONDirPath)
		}
		opts := []string{
			"Yes",
			"No, create package.json in my working directory",
		}
		useExisting := opts[0]
		var surveyResp string
		if err := survey.AskOne(
			&survey.Select{
				Message: fmt.Sprintf("Found existing package.json in %s. Use this to manage dependencies for your view?", packageJSONDirPath),
				Options: opts,
				Default: useExisting,
			},
			&surveyResp,
		); err != nil {
			return err
		}
		if surveyResp == useExisting {
			return yarnAddViewsPackage(packageJSONDirPath)
		}
	}
	// Create package.json in cwd and install
	cmd := exec.Command("yarn", "init", "-y")
	cmd.Dir = cwd
	if err := cmd.Start(); err != nil {

		return errors.Wrap(err, "yarn init")
	}
	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "yarn init wait")
	}
	logger.Step("Created package.json")

	return yarnAddViewsPackage(cwd)
}

func yarnAddViewsPackage(packageJSONDirPath string) error {
	logger.Step("Installing @airplane/views...")

	cmd := exec.Command("yarn", "add", "@airplane/views")
	cmd.Dir = packageJSONDirPath
	out, err := cmd.CombinedOutput()

	logger.Log(string(out))

	if err != nil {
		logger.Log("Failed to install @airplane/views")
		return err
	}
	logger.Step("Installed @airplane/views")
	return nil
}
