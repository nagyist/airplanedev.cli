package initcmd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/airplanedev/cli/pkg/deploy/taskdir/definitions"
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
	Logger           logger.Logger
	DryRun           bool
	WorkingDirectory string

	Name        string
	Slug        string
	Description string

	// ease of testing
	suffixCharset string
}

func InitView(ctx context.Context, req InitViewRequest) (InitResponse, error) {
	if req.suffixCharset == "" {
		req.suffixCharset = utils.CharsetLowercaseNumeric
	}
	if req.Logger == nil {
		req.Logger = logger.NewNoopLogger()
	}
	if req.Name == "" {
		return InitResponse{}, errors.New("missing new view name")
	}

	if req.WorkingDirectory == "" {
		wd, err := filepath.Abs(".")
		if err != nil {
			return InitResponse{}, err
		}
		req.WorkingDirectory = wd
	} else {
		wd, err := filepath.Abs(req.WorkingDirectory)
		if err != nil {
			return InitResponse{}, err
		}
		req.WorkingDirectory = wd
	}
	ret, err := newInitResponse(req.WorkingDirectory)
	if err != nil {
		return InitResponse{}, err
	}

	if req.WorkingDirectory == "" {
		wd, err := filepath.Abs(".")
		if err != nil {
			return InitResponse{}, err
		}
		req.WorkingDirectory = wd
	} else {
		wd, err := filepath.Abs(req.WorkingDirectory)
		if err != nil {
			return InitResponse{}, err
		}
		req.WorkingDirectory = wd
	}

	slug := req.Slug
	if slug == "" {
		slug = utils.MakeSlug(req.Name)
	}
	viewDir := ""

	entrypoint, err := createViewEntrypoint(createViewEntrypointRequest{
		Logger:           req.Logger,
		DryRun:           req.DryRun,
		WorkingDirectory: req.WorkingDirectory,
		Slug:             slug,
		Name:             req.Name,
		Description:      req.Description,
		suffixCharset:    req.suffixCharset,
	})
	if err != nil {
		return InitResponse{}, err
	}
	ret.AddCreatedFile(entrypoint)

	packageJSONDir := req.WorkingDirectory
	if fsx.Exists(filepath.Join(req.WorkingDirectory, viewDir, "package.json")) {
		packageJSONDir = filepath.Join(req.WorkingDirectory, viewDir)
	}
	deps := []string{"@airplane/views", "react", "react-dom"}
	deps = append(deps, "airplane")
	packageJSONDir, packageJSONCreated, err := node.CreatePackageJSON(packageJSONDir, node.PackageJSONOptions{
		Dependencies: node.NodeDependencies{
			Dependencies:    deps,
			DevDependencies: []string{"@types/react", "@types/react-dom", "typescript"},
		},
	}, req.Prompter, req.Logger, req.DryRun)
	if err != nil {
		return InitResponse{}, err
	}
	ret.AddFile(packageJSONCreated, path.Join(packageJSONDir, "package.json"))

	if filepath.Ext(entrypoint) == ".tsx" {
		// Create/update tsconfig in the same directory as the package.json file
		tsConfigCreated, err := node.CreateViewTSConfig(packageJSONDir, req.Prompter, req.Logger, req.DryRun)
		if err != nil {
			return InitResponse{}, err
		}
		ret.AddFile(tsConfigCreated, path.Join(packageJSONDir, "tsconfig.json"))
	}

	gitignorePath := filepath.Join(req.WorkingDirectory, ".gitignore")
	if utils.ShouldCreateDefaultGitignoreFile(gitignorePath) {
		if !req.DryRun {
			if err := utils.CreateDefaultGitignoreFile(gitignorePath); err != nil {
				return InitResponse{}, err
			}
		}
		ret.AddCreatedFile(gitignorePath)
	}

	if req.DryRun {
		req.Logger.Log("Running with --dry-run. This command would have created or updated the following files:\n%s", ret.String())
	}

	suggestNextViewSteps(suggestNextViewStepsRequest{
		logger:  req.Logger,
		viewDir: viewDir,
		slug:    slug,
	})

	ret.NewViewDefinition = &definitions.ViewDefinition{
		Name:         req.Name,
		Slug:         slug,
		Description:  req.Description,
		DefnFilePath: entrypoint,
	}

	return ret, nil
}

func generateViewEntrypointPath(slug string) string {
	return fmt.Sprintf("%s.airplane.tsx", strcase.ToCamel(slug))
}

//go:embed view_scaffolding/Default.airplane.tsx
var defaultViewEntrypointInline []byte

type createViewEntrypointRequest struct {
	Logger           logger.Logger
	DryRun           bool
	WorkingDirectory string
	Slug             string
	Name             string
	Description      string
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
		"ViewName":    strcase.ToCamel(req.Slug),
		"Slug":        req.Slug,
		"Name":        req.Name,
		"Description": req.Description,
	}); err != nil {
		return "", errors.Wrap(err, "executing inline entrypoint template")
	}
	entrypointContents = buf.Bytes()
	if !req.DryRun {
		if err := os.WriteFile(absEntrypointPath, entrypointContents, 0644); err != nil {
			return "", errors.Wrap(err, "creating view entrypoint")
		}
	}
	req.Logger.Step("Created view entrypoint at %s", absEntrypointPath)
	return absEntrypointPath, nil
}
