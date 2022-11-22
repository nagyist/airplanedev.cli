package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	deployutils "github.com/airplanedev/lib/pkg/deploy/utils"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

type CodeViewDiscoverer struct {
	Client api.IAPIClient
	Logger logger.Logger

	MissingViewHandler func(context.Context, definitions.ViewDefinition) (*api.View, error)

	// DoNotVerifyMissingViews will return ViewConfigs for views without verifying their existence
	// in the api. If this value is set to true, MissingViewHandler is ignored.
	DoNotVerifyMissingViews bool
}

var _ ViewDiscoverer = &CodeViewDiscoverer{}

func (dd *CodeViewDiscoverer) GetViewConfig(ctx context.Context, file string) (*ViewConfig, error) {
	if !deployutils.IsViewInlineAirplaneEntity(file) {
		return nil, nil
	}

	pm, err := taskPathMetadata(file, build.TaskKindNode)
	if err != nil {
		return nil, err
	}

	if err := esbuildUserFiles(pm.RootDir); err != nil {
		// TODO: convert to an error once inline discovery is more stable.
		dd.Logger.Warning(`Unable to build view: %s`, err.Error())
		return nil, nil
	}
	defer func() {
		if err := os.RemoveAll(path.Join(pm.RootDir, ".airplane", "discover")); err != nil {
			dd.Logger.Warning("unable to remove temporary directory: %s")
		}
	}()

	compiledJSPath, err := compiledFilePath(pm.RootDir, file)
	if err != nil {
		return nil, err
	}

	parsedConfigs, err := extractJSConfigs(compiledJSPath)
	if err != nil {
		dd.Logger.Warning(`Unable to discover inline configured views: %s`, err.Error())
	}

	if len(parsedConfigs.ViewConfigs) == 0 {
		return nil, nil
	}
	if len(parsedConfigs.ViewConfigs) > 1 {
		return nil, errors.New(fmt.Sprintf("unable to parse multiple views from %s, do not export more than one view", file))
	}

	d := definitions.ViewDefinition{}
	parsedConfigs.ViewConfigs[0]["entrypoint"] = pm.AbsEntrypoint
	buf, err := json.Marshal(parsedConfigs.ViewConfigs[0])
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize view json properly")
	}

	if err = d.Unmarshal(definitions.DefFormatJSON, buf); err != nil {
		switch err := errors.Cause(err).(type) {
		case definitions.ErrSchemaValidation:
			errorMsgs := []string{}
			for _, verr := range err.Errors {
				errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", verr.Field(), verr.Description()))
			}
			return nil, definitions.NewErrReadDefinition(fmt.Sprintf("Error reading %s", file), errorMsgs...)
		default:
			return nil, errors.Wrap(err, "unmarshalling view definition")
		}
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

	d.DefnFilePath = pm.AbsEntrypoint

	return &ViewConfig{
		ID:     view.ID,
		Def:    d,
		Source: ConfigSourceCode,
		Root:   pm.RootDir,
	}, nil
}

func (dd *CodeViewDiscoverer) GetViewRoot(ctx context.Context, file string) (string, build.BuildType, build.BuildTypeVersion, build.BuildBase, error) {
	if !deployutils.IsViewInlineAirplaneEntity(file) {
		return "", "", "", "", nil
	}
	root, err := filepath.Abs(filepath.Dir(file))
	if err != nil {
		return "", "", "", "", errors.Wrap(err, "getting absolute view definition root")
	}
	if p, ok := fsx.Find(root, "package.json"); ok {
		root = p
	}

	pm, err := taskPathMetadata(file, build.TaskKindNode)
	if err != nil {
		return "", "", "", "", errors.Wrap(err, "unable to interpret task path metadata")
	}

	return root, build.ViewBuildType, pm.BuildVersion, pm.BuildBase, nil
}

func (dd *CodeViewDiscoverer) ConfigSource() ConfigSource {
	return ConfigSourceCode
}
