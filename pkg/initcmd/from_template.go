package initcmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/airplanedev/cli/pkg/analytics"
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
)

type InitFromTemplateRequest struct {
	Client   api.APIClient
	Prompter prompts.Prompter
	Logger   logger.Logger

	TemplateSlug string
	Templates    []Template
}

type Template struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	GitHubPath    string   `json:"githubPath"`
	DemoResources []string `json:"demoResources"`
	ViewSlugs     []string `json:"viewSlugs"`
	TaskSlugs     []string `json:"taskSlugs"`
}

func InitFromTemplate(ctx context.Context, req InitFromTemplateRequest) error {
	analytics.Track(req.Client, "Template Cloned", map[string]interface{}{
		"template_path": req.TemplateSlug,
	})

	var templatePath string
	if strings.HasPrefix(req.TemplateSlug, "github.com/") || strings.HasPrefix(req.TemplateSlug, "https://github.com/") {
		templatePath = req.TemplateSlug
	} else {
		if len(req.Templates) == 0 {
			templates, err := ListTemplates(ctx)
			if err != nil {
				return err
			}
			req.Templates = templates
		}
		template, err := FindTemplate(req.Templates, req.TemplateSlug)
		if err != nil {
			return err
		}
		templatePath = template.GitHubPath
	}

	return utils.CopyFromGithubPath(req.Prompter, req.Logger, templatePath)
}

const docsUrl = "http://docs.airplane.dev/templates/templates.json"
const defaultGitPrefix = "github.com/airplanedev/templates"

func ListTemplates(ctx context.Context) ([]Template, error) {
	//nolint: noctx
	resp, err := http.Get(docsUrl)
	if err != nil {
		return []Template{}, errors.Wrap(err, "getting templates json")
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return []Template{}, errors.Wrap(err, "reading templates")
	}

	var t []Template
	if err = json.Unmarshal(buf, &t); err != nil {
		return []Template{}, errors.Wrap(err, "unmarshalling templates")
	}
	return t, nil
}

func FindTemplate(templates []Template, gitPath string) (Template, error) {
	if !strings.HasPrefix(gitPath, "github.com/") {
		if strings.HasPrefix(gitPath, "https://github.com/") {
			gitPath = strings.TrimPrefix(gitPath, "https://")
		} else {
			p, err := url.JoinPath(defaultGitPrefix, gitPath)
			if err != nil {
				return Template{}, errors.Wrapf(err, "creating URL from %s and %s", defaultGitPrefix, gitPath)
			}
			gitPath = p
		}
	}

	for _, t := range templates {
		if t.GitHubPath == gitPath {
			return t, nil
		}
	}
	return Template{}, errors.New("template not found")
}
