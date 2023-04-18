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
	Prompter         prompts.Prompter
	DryRun           bool
	WorkingDirectory string

	Name string

	// ease of testing
	suffixCharset string
}

func InitView(ctx context.Context, req InitViewRequest) ([]string, error) {
	if req.suffixCharset == "" {
		req.suffixCharset = utils.CharsetLowercaseNumeric
	}
	filesCreated := []string{}
	if req.Name == "" {
		return nil, errors.New("missing new view name")
	}

	if req.WorkingDirectory == "" {
		wd, err := filepath.Abs(".")
		if err != nil {
			return nil, err
		}
		req.WorkingDirectory = wd
	} else {
		wd, err := filepath.Abs(req.WorkingDirectory)
		if err != nil {
			return nil, err
		}
		req.WorkingDirectory = wd
	}

	slug := utils.MakeSlug(req.Name)
	viewDir := ""

	entrypoint, err := createViewEntrypoint(createViewEntrypointRequest{
		DryRun:           req.DryRun,
		WorkingDirectory: req.WorkingDirectory,
		Slug:             slug,
		Name:             req.Name,
		suffixCharset:    req.suffixCharset,
	})
	if err != nil {
		return nil, err
	}
	filesCreated = append(filesCreated, entrypoint)

	packageJSONDir := req.WorkingDirectory
	if fsx.Exists(filepath.Join(req.WorkingDirectory, viewDir, "package.json")) {
		packageJSONDir = filepath.Join(req.WorkingDirectory, viewDir)
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
		if err := utils.CreateDefaultGitignoreFile(filepath.Join(req.WorkingDirectory, ".gitignore")); err != nil {
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

type createViewEntrypointRequest struct {
	DryRun           bool
	WorkingDirectory string
	Slug             string
	Name             string
	suffixCharset    string
}

func createViewEntrypoint(req createViewEntrypointRequest) (string, error) {
	entrypointPath := generateViewEntrypointPath(req.Slug)
	absEntrypointPath, err := trySuffix(filepath.Join(req.WorkingDirectory, entrypointPath), nil, 3, req.suffixCharset)
	if err != nil {
		return "", err
	}

	var entrypointContents []byte
	tmpl, err := template.New("entrypoint").Parse(string(defaultViewEntrypointInline))
	if err != nil {
		return "", errors.Wrap(err, "parsing inline entrypoint template")
	}
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, map[string]interface{}{
		"ViewName": strcase.ToCamel(req.Slug),
		"Slug":     req.Slug,
		"Name":     req.Name,
	}); err != nil {
		return "", errors.Wrap(err, "executing inline entrypoint template")
	}
	entrypointContents = buf.Bytes()
	if !req.DryRun {
		if err := os.WriteFile(absEntrypointPath, entrypointContents, 0644); err != nil {
			return "", errors.Wrap(err, "creating view entrypoint")
		}
	}
	logger.Step("Created view entrypoint at %s", absEntrypointPath)
	return absEntrypointPath, nil
}
