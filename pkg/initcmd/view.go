package initcmd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/node"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
)

type InitViewRequest struct {
	Prompter prompts.Prompter
	DryRun   bool

	Name string
}

func InitView(ctx context.Context, req InitViewRequest) ([]string, error) {
	filesCreated := []string{}
	if req.Name == "" {
		return nil, errors.New("missing new view name")
	}

	slug := utils.MakeSlug(req.Name)
	viewDir := ""

	entrypoint, err := createViewEntrypoint(req.DryRun, slug, req.Name)
	if err != nil {
		return nil, err
	}
	filesCreated = append(filesCreated, entrypoint)

	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "getting working directory")
	}
	packageJSONDir := cwd
	if fsx.Exists(filepath.Join(cwd, viewDir, "package.json")) {
		packageJSONDir = filepath.Join(cwd, viewDir)
	}
	if !req.DryRun {
		deps := []string{"@airplane/views", "react", "react-dom"}
		deps = append(deps, "airplane")
		packageJSONDir, err = node.CreatePackageJSON(packageJSONDir, node.PackageJSONOptions{
			Dependencies: node.NodeDependencies{
				Dependencies:    deps,
				DevDependencies: []string{"@types/react", "@types/react-dom", "typescript"},
			},
		}, req.Prompter)
		if err != nil {
			return nil, err
		}
	}
	filesCreated = append(filesCreated, path.Join(packageJSONDir, "package.json"))

	if filepath.Ext(entrypoint) == ".tsx" {
		if !req.DryRun {
			// Create/update tsconfig in the same directory as the package.json file
			if err := node.CreateViewTSConfig(packageJSONDir, req.Prompter); err != nil {
				return nil, err
			}
		}
		filesCreated = append(filesCreated, path.Join(packageJSONDir, "tsconfig.json"))
	}

	if !req.DryRun {
		if err := utils.CreateDefaultGitignoreFile(".gitignore"); err != nil {
			return nil, err
		}
	}
	filesCreated = append(filesCreated, ".gitignore")

	if req.DryRun {
		logger.Log("Running with --dry-run. This command would have created or updated the following files:\n- %s", strings.Join(filesCreated, "\n- "))
	}

	suggestNextViewSteps(suggestNextViewStepsRequest{
		viewDir: viewDir,
		slug:    slug,
	})

	return filesCreated, nil
}

func generateViewEntrypointPath(slug string) string {
	return fmt.Sprintf("%s.airplane.tsx", strcase.ToCamel(slug))
}

//go:embed view_scaffolding/Default.airplane.tsx
var defaultViewEntrypointInline []byte

func createViewEntrypoint(dryRun bool, slug string, name string) (string, error) {
	entrypointPath := generateViewEntrypointPath(slug)

	var entrypointContents []byte
	tmpl, err := template.New("entrypoint").Parse(string(defaultViewEntrypointInline))
	if err != nil {
		return "", errors.Wrap(err, "parsing inline entrypoint template")
	}
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, map[string]interface{}{
		"ViewName": strcase.ToCamel(slug),
		"Slug":     slug,
		"Name":     name,
	}); err != nil {
		return "", errors.Wrap(err, "executing inline entrypoint template")
	}
	entrypointContents = buf.Bytes()
	if !dryRun {
		if err := os.WriteFile(entrypointPath, entrypointContents, 0644); err != nil {
			return "", errors.Wrap(err, "creating view entrypoint")
		}
	}
	logger.Step("Created view entrypoint at %s", entrypointPath)
	return entrypointPath, nil
}
