package discover

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/api"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

type ViewDefnDiscoverer struct {
	Client api.IAPIClient
	Logger logger.Logger

	MissingViewHandler func(context.Context, definitions.ViewDefinition) (*api.View, error)
	// DoNotVerifyMissingViews will return ViewConfigs for views without verifying their existence
	// in the api. If this value is set to true, MissingViewHandler is ignored.
	DoNotVerifyMissingViews bool
}

var _ ViewDiscoverer = &ViewDefnDiscoverer{}

func (dd *ViewDefnDiscoverer) GetViewConfig(ctx context.Context, file string) (*ViewConfig, error) {
	if !definitions.IsViewDef(file) {
		return nil, nil
	}

	d, err := getViewDefinitionFromFile(file)
	if err != nil {
		return nil, err
	}

	root, _, err := dd.GetViewRoot(ctx, file)
	if err != nil {
		return nil, err
	}
	bc, err := ViewBuildContext(root)
	if err != nil {
		return nil, err
	}
	d.Base = bc.Base

	envVars := make(api.EnvVars)
	envVarsFromDefn := d.EnvVars
	// Calculate the full list of env vars. This is the env vars (from airplane config)
	// plus the env vars from the view. Set this new list on the def.
	for k, v := range bc.EnvVars {
		envVars[k] = api.EnvVarValue(v)
	}
	for k, v := range envVarsFromDefn {
		envVars[k] = v
	}
	if len(envVars) > 0 {
		d.EnvVars = envVars
	}

	var view api.View
	if !dd.DoNotVerifyMissingViews {
		view, err = dd.Client.GetView(ctx, api.GetViewRequest{Slug: d.Slug})
		if err != nil {
			var merr *api.ViewMissingError
			if !errors.As(err, &merr) {
				return nil, errors.Wrap(err, "unable to get view")
			}
			if dd.MissingViewHandler == nil {
				return nil, nil
			}

			vptr, err := dd.MissingViewHandler(ctx, d)
			if err != nil {
				return nil, err
			} else if vptr == nil {
				if dd.Logger != nil {
					dd.Logger.Warning(`View with slug %s does not exist, skipping deployment.`, d.Slug)
				}
				return nil, nil
			}
			view = *vptr
		}
		if view.ArchivedAt != nil {
			dd.Logger.Warning(`View with slug %s is archived, skipping deployment.`, view.Slug)
			return nil, nil
		}
	}

	return &ViewConfig{
		ID:     view.ID,
		Def:    d,
		Source: dd.ConfigSource(),
		Root:   root,
	}, nil
}

func (dd *ViewDefnDiscoverer) GetViewRoot(ctx context.Context, file string) (string, buildtypes.BuildContext, error) {
	if !definitions.IsViewDef(file) {
		return "", buildtypes.BuildContext{}, nil
	}

	d, err := getViewDefinitionFromFile(file)
	if err != nil {
		return "", buildtypes.BuildContext{}, err
	}

	root, err := filepath.Abs(filepath.Dir(file))
	if err != nil {
		return "", buildtypes.BuildContext{}, errors.Wrap(err, "getting absolute view definition root")
	}
	if p, ok := fsx.Find(root, "package.json"); ok {
		root = p
	}

	pm, err := taskPathMetadata(d.Entrypoint, buildtypes.TaskKindNode)
	if err != nil {
		return "", buildtypes.BuildContext{}, err
	}
	bc, err := ViewBuildContext(pm.RootDir)
	if err != nil {
		return "", buildtypes.BuildContext{}, err
	}

	return root, buildtypes.BuildContext{
		Type:    buildtypes.ViewBuildType,
		Version: bc.Version,
		Base:    bc.Base,
		EnvVars: bc.EnvVars,
	}, nil
}

func (dd *ViewDefnDiscoverer) ConfigSource() ConfigSource {
	return ConfigSourceDefn
}

func getViewDefinitionFromFile(file string) (definitions.ViewDefinition, error) {
	buf, err := os.ReadFile(file)
	if err != nil {
		return definitions.ViewDefinition{}, errors.Wrap(err, "reading view definition")
	}
	format := definitions.GetViewDefFormat(file)

	d := definitions.ViewDefinition{}
	if err := d.Unmarshal(format, buf); err != nil {
		switch err := errors.Cause(err).(type) {
		case definitions.ErrSchemaValidation:
			errorMsgs := []string{}
			for _, verr := range err.Errors {
				errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", verr.Field(), verr.Description()))
			}
			return definitions.ViewDefinition{}, definitions.NewErrReadDefinition(fmt.Sprintf("Error reading %s", file), errorMsgs...)
		default:
			return definitions.ViewDefinition{}, errors.Wrap(err, "unmarshalling view definition")
		}
	}
	d.DefnFilePath, err = filepath.Abs(file)
	if err != nil {
		return definitions.ViewDefinition{}, errors.Wrap(err, "getting absolute path of view definition file")
	}
	if !filepath.IsAbs(d.Entrypoint) {
		defnDir := filepath.Dir(file)
		d.Entrypoint, err = filepath.Abs(filepath.Join(defnDir, d.Entrypoint))
		if err != nil {
			return definitions.ViewDefinition{}, err
		}
	}
	return d, nil
}
