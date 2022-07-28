package initcmd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/build"
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
	slug    string
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
	slug := utils.MakeSlug(cfg.name)
	cfg.slug = slug

	// Default to creating folder with the slug
	directory := slug
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
	if err := createTSConfigFile(); err != nil {
		return err
	}
	if err := createPackageJSON(*cfg); err != nil {
		return err
	}
	suggestNextSteps(*cfg)
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

func shouldUseYarn(packageJSONDirPath string) bool {
	// If the closest directory with a package.json has a lockfile, we will use that to
	// determine whether to use yarn or npm even if we eventually create a new package.json for the view.
	yarnlock := filepath.Join(packageJSONDirPath, "yarn.lock")
	pkglock := filepath.Join(packageJSONDirPath, "package-lock.json")

	if err := fsx.AssertExistsAll(yarnlock); err == nil {
		return true
	} else if err := fsx.AssertExistsAll(pkglock); err == nil {
		return false
	}

	// No lockfiles, so check if yarn is installed by getting yarn version
	cmd := exec.Command("yarn", "-v")
	cmd.Dir = filepath.Dir(packageJSONDirPath)
	err := cmd.Start()
	return err == nil
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
	useYarn := shouldUseYarn(packageJSONDirPath)

	if ok {
		if packageJSONDirPath == cwd {
			return addAllPackages(packageJSONDirPath, useYarn)
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
			return addAllPackages(packageJSONDirPath, useYarn)
		}
	}

	if err := createPackageJSONFile(cwd); err != nil {
		return err
	}
	return addAllPackages(cwd, useYarn)
}

func addAllPackages(packageJSONDirPath string, useYarn bool) error {
	packageJSONPath := filepath.Join(packageJSONDirPath, "package.json")
	existingDeps, err := build.ListDependencies(packageJSONPath)
	if err != nil {
		return err
	}
	// TODO: Select versions to install instead of installing latest.
	// Put these in lib and use same ones for airplane views dev.
	packagesToCheck := []string{"@airplane/views", "react", "react-dom"}
	packagesToAdd := getPackagesToAdd(packagesToCheck, existingDeps)

	devPackagesToCheck := []string{"@types/react", "@types/react-dom", "typescript"}
	devPackagesToAdd := getPackagesToAdd(devPackagesToCheck, existingDeps)

	if len(packagesToAdd) > 0 || len(devPackagesToAdd) > 0 {
		logger.Step("Installing dependencies...")
	}

	if len(packagesToAdd) > 0 {
		if err := addPackages(packageJSONDirPath, packagesToAdd, false, useYarn); err != nil {
			return errors.Wrap(err, "installing dependencies")
		}
	}

	if len(devPackagesToAdd) > 0 {
		if err := addPackages(packageJSONDirPath, devPackagesToAdd, true, useYarn); err != nil {
			return errors.Wrap(err, "installing dev dependencies")
		}
	}
	return nil
}

func getPackagesToAdd(packagesToCheck, existingDeps []string) []string {
	packagesToAdd := []string{}
	for _, pkg := range packagesToCheck {
		hasPackage := false
		for _, d := range existingDeps {
			if d == pkg {
				hasPackage = true
				break
			}
		}
		if !hasPackage {
			packagesToAdd = append(packagesToAdd, pkg)
		}
	}
	return packagesToAdd
}

func addPackages(packageJSONDirPath string, packageNames []string, dev, useYarn bool) error {
	installArgs := []string{"add"}
	if dev {
		if useYarn {
			installArgs = append(installArgs, "--dev")
		} else {
			installArgs = append(installArgs, "--save-dev")
		}
	}
	installArgs = append(installArgs, packageNames...)
	var cmd *exec.Cmd
	if useYarn {
		cmd = exec.Command("yarn", installArgs...)
	} else {
		cmd = exec.Command("npm", installArgs...)
	}

	cmd.Dir = packageJSONDirPath
	err := cmd.Run()
	if err != nil {
		if dev {
			logger.Log("Failed to install devDependencies")
		} else {
			logger.Log("Failed to install dependencies")
		}
		return err
	}
	for _, pkg := range packageNames {
		logger.Step(fmt.Sprintf("Installed %s", pkg))
	}
	return nil
}

//go:embed scaffolding/package.json
var packageJsonTemplateStr string

func createPackageJSONFile(cwd string) error {
	tmpl, err := template.New("packageJson").Parse(packageJsonTemplateStr)
	if err != nil {
		return errors.Wrap(err, "parsing package.json template")
	}
	normalizedCwd := strings.ReplaceAll(strings.ToLower(filepath.Base(cwd)), " ", "-")
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, map[string]interface{}{
		"name": normalizedCwd,
	}); err != nil {
		return errors.Wrap(err, "executing package.json template")
	}

	if err := ioutil.WriteFile("package.json", buf.Bytes(), 0644); err != nil {
		return errors.Wrap(err, "writing package.json")
	}
	logger.Step("Created package.json")
	return nil
}

//go:embed scaffolding/tsconfig.json
var defaultTSConfig []byte

func createTSConfigFile() error {
	if !fsx.Exists("tsconfig.json") {
		if err := ioutil.WriteFile("tsconfig.json", defaultTSConfig, 0644); err != nil {
			return errors.Wrap(err, "creating tsconfig.json")
		}
		logger.Step("Created tsconfig.json")
	}
	return nil
}

func suggestNextSteps(cfg config) {
	logger.Suggest("✅ To complete your view:", fmt.Sprintf("Write your view logic in %s", generateEntrypointPath(cfg, false)))

	logger.Suggest(
		"⚡ To run the view locally:",
		"airplane views dev %s",
		cfg.slug,
	)
	logger.Suggest(
		"🛫 To deploy your view to Airplane:",
		"airplane deploy %s",
		cfg.slug,
	)
}
