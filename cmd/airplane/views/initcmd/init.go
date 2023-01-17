package initcmd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/flags/flagsiface"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/node"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	client  *api.Client
	root    *cli.Config
	inline  bool
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
	cmd.Flags().BoolVar(&cfg.inline, "inline", false, "If true, the view will be configured with inline configuration")
	cfg.cmd = cmd

	return cmd
}

func GetConfig(c *cli.Config) config {
	return config{client: c.Client, root: c}
}

func Run(ctx context.Context, cfg config) error {
	inlineSetByUser := cfg.cmd != nil && cfg.cmd.Flags().Changed("inline")
	if !inlineSetByUser {
		if cfg.root.Flagger.Bool(ctx, logger.NewStdErrLogger(logger.StdErrLoggerOpts{}), flagsiface.DefaultInlineConfigViews) {
			cfg.inline = true
		}
	}
	if cfg.from != "" {
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

	var directory string
	if !cfg.inline {
		// Nest non-inline views in a folder.
		directory = slug
		if err := utils.CreateDirectory(directory); err != nil {
			return err
		}
	}
	cfg.viewDir = directory

	if !cfg.inline {
		if err := createViewDefinition(*cfg); err != nil {
			return err
		}
	}
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
	if cfg.inline {
		deps = append(deps, "airplane")
	}
	packageJSONDir, err = node.CreatePackageJSON(packageJSONDir, node.PackageJSONOptions{
		Dependencies: node.NodeDependencies{
			Dependencies:    deps,
			DevDependencies: []string{"@types/react", "@types/react-dom", "typescript"},
		},
	})
	if err != nil {
		return err
	}
	if filepath.Ext(entrypoint) == ".tsx" {
		// Create/update tsconfig in the same directory as the package.json file
		if err := node.CreateViewTSConfig(packageJSONDir); err != nil {
			return err
		}
	}

	if err := utils.CreateDefaultGitignoreFile(".gitignore"); err != nil {
		return err
	}
	return nil
}

func generateEntrypointPath(cfg config, inViewDir bool) string {
	if inViewDir {
		return fmt.Sprintf("%s.view.tsx", cfg.slug)
	} else if !cfg.inline {
		return fmt.Sprintf("%s/%s.view.tsx", cfg.viewDir, cfg.slug)
	} else {
		return fmt.Sprintf("%s.airplane.tsx", cfg.slug)
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

	if err := os.WriteFile(defnFilename, buf, 0644); err != nil {
		return err
	}
	logger.Step("Created view definition at %s", defnFilename)
	return nil
}

//go:embed scaffolding/default.view.tsx
var defaultEntrypoint []byte

//go:embed scaffolding/default_inline.airplane.tsx
var defaultEntrypointInline []byte

func createEntrypoint(cfg config) (string, error) {
	entrypointPath := generateEntrypointPath(cfg, false)

	var entrypointContents []byte
	if cfg.inline {
		tmpl, err := template.New("entrypoint").Parse(string(defaultEntrypointInline))
		if err != nil {
			return "", errors.Wrap(err, "parsing entrypoint template")
		}
		buf := new(bytes.Buffer)
		if err := tmpl.Execute(buf, map[string]interface{}{
			"Slug": cfg.slug,
			"Name": cfg.name,
		}); err != nil {
			return "", errors.Wrap(err, "executing entrypoint template")
		}
		entrypointContents = buf.Bytes()
	} else {
		entrypointContents = defaultEntrypoint
	}
	if err := os.WriteFile(entrypointPath, entrypointContents, 0644); err != nil {
		return "", errors.Wrap(err, "creating view entrypoint")
	}
	logger.Step("Created view entrypoint at %s", entrypointPath)
	return entrypointPath, nil
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
