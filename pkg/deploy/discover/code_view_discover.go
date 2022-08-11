package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

type CodeViewDiscoverer struct {
	Client api.IAPIClient
	Logger logger.Logger

	MissingViewHandler func(context.Context, definitions.ViewDefinition) (*api.View, error)
}

var _ ViewDiscoverer = &CodeViewDiscoverer{}

func (dd *CodeViewDiscoverer) GetViewConfig(ctx context.Context, file string) (*ViewConfig, error) {
	if !isCodeViewFile(file) {
		return nil, nil
	}

	pm, err := taskPathMetadata(file, build.TaskKindNode)
	if err != nil {
		return nil, err
	}

	if err := esbuildUserFiles(pm.RootDir); err != nil {
		return nil, err
	}
	defer func() {
		if err := os.RemoveAll(path.Join(pm.RootDir, ".airplane")); err != nil {
			dd.Logger.Warning("unable to remove temporary directory: %s")
		}
	}()

	cleanup, err := maybePatchNodeModules(dd.Logger, pm.RootDir)
	defer cleanup()
	if err != nil {
		return nil, err
	}

	compiledJSPath, err := compiledFilePath(pm.RootDir, file)
	if err != nil {
		return nil, err
	}

	parsedConfigs, err := extractConfigs(compiledJSPath)
	if err != nil {
		return nil, err
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

	view, err := dd.Client.GetView(ctx, api.GetViewRequest{Slug: d.Slug})
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

	return &ViewConfig{
		ID:     view.ID,
		Def:    d,
		Source: ConfigSourceCode,
		Root:   pm.RootDir,
	}, nil
}

func (dd *CodeViewDiscoverer) ConfigSource() ConfigSource {
	return ConfigSourceCode
}

func isCodeViewFile(file string) bool {
	for _, suffix := range []string{".view.tsx", ".view.jsx"} {
		if strings.HasSuffix(file, suffix) {
			return true
		}
	}
	return false
}
