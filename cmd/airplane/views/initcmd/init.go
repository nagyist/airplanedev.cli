package initcmd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/node"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	root    *cli.Config
	name    string
	viewDir string
	slug    string
	from    string
	cmd     *cobra.Command
}

func New(c *cli.Config) *cobra.Command {
	var cfg = GetConfig(c)

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Initialize a view definition",
		Example: heredoc.Doc("$ airplane views init"),
		// TODO: support passing in where to create the directory either as arg or flag
		Args: cobra.MaximumNArgs(0),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd.Root().Context(), cfg)
		},
	}
	cmd.Flags().StringVar(&cfg.from, "from", "", "Path to an existing github URL to initialize from")
	cfg.cmd = cmd

	return cmd
}

func GetConfig(c *cli.Config) config {
	return config{root: c}
}

func Run(ctx context.Context, cfg config) error {
	if cfg.from != "" {
		if err := utils.CopyFromGithubPath(cfg.from, cfg.root.Prompter); err != nil {
			return err
		}
		(&cfg).viewDir = filepath.Base(cfg.from)
	} else {
		if err := promptForNewView(&cfg); err != nil {
			return err
		}
		if err := createViewScaffolding(&cfg); err != nil {
			return err
		}
	}
	suggestNextSteps(cfg)
	return nil
}

func promptForNewView(config *config) error {
	return config.root.Prompter.Input("What should this view be called?", &config.name)
}

func createViewScaffolding(cfg *config) error {
	if cfg.name == "" {
		return errors.New("missing new view name")
	}

	slug := utils.MakeSlug(cfg.name)
	cfg.slug = slug

	var directory string
	cfg.viewDir = directory

	entrypoint, err := createEntrypoint(*cfg)
	if err != nil {
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
	deps := []string{"@airplane/views", "react", "react-dom"}
	deps = append(deps, "airplane")
	packageJSONDir, err = node.CreatePackageJSON(packageJSONDir, node.PackageJSONOptions{
		Dependencies: node.NodeDependencies{
			Dependencies:    deps,
			DevDependencies: []string{"@types/react", "@types/react-dom", "typescript"},
		},
	}, cfg.root.Prompter)
	if err != nil {
		return err
	}
	if filepath.Ext(entrypoint) == ".tsx" {
		// Create/update tsconfig in the same directory as the package.json file
		if err := node.CreateViewTSConfig(packageJSONDir, cfg.root.Prompter); err != nil {
			return err
		}
	}

	if err := utils.CreateDefaultGitignoreFile(".gitignore"); err != nil {
		return err
	}
	return nil
}

func generateEntrypointPath(cfg config) string {
	return fmt.Sprintf("%s.airplane.tsx", strcase.ToCamel(cfg.slug))
}

//go:embed scaffolding/Default.airplane.tsx
var defaultEntrypointInline []byte

func createEntrypoint(cfg config) (string, error) {
	entrypointPath := generateEntrypointPath(cfg)

	var entrypointContents []byte
	tmpl, err := template.New("entrypoint").Parse(string(defaultEntrypointInline))
	if err != nil {
		return "", errors.Wrap(err, "parsing inline entrypoint template")
	}
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, map[string]interface{}{
		"ViewName": strcase.ToCamel(cfg.slug),
		"Slug":     cfg.slug,
		"Name":     cfg.name,
	}); err != nil {
		return "", errors.Wrap(err, "executing inline entrypoint template")
	}
	entrypointContents = buf.Bytes()
	if err := os.WriteFile(entrypointPath, entrypointContents, 0644); err != nil {
		return "", errors.Wrap(err, "creating view entrypoint")
	}
	logger.Step("Created view entrypoint at %s", entrypointPath)
	return entrypointPath, nil
}

func suggestNextSteps(cfg config) {
	if cfg.viewDir != "" && cfg.slug != "" {
		logger.Suggest("✅ To complete your view:", fmt.Sprintf("Write your view logic in %s", generateEntrypointPath(cfg)))
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
