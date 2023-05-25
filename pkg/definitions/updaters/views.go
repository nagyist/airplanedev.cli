package updaters

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/definitions"
	deployutils "github.com/airplanedev/cli/pkg/deploy/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

func UpdateView(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.ViewDefinition) error {
	if deployutils.IsNodeInlineAirplaneEntity(path) {
		if _, err := os.Stat(path); err != nil {
			return errors.Wrap(err, "opening file")
		}

		defJSON, err := def.Marshal(definitions.DefFormatJSON)
		if err != nil {
			return errors.Wrap(err, "marshalling definition as JSON")
		}

		_, err = runNodeCommand(ctx, logger, javaScriptEntrypoint, "update_view", path, slug, string(defJSON))
		if err != nil {
			return errors.WithMessagef(err, "updating view at %q (re-run with --debug for more context)", path)
		}

		return nil
	}

	return updateYAMLView(ctx, logger, path, slug, def)
}

func CanUpdateView(ctx context.Context, logger logger.Logger, path string, slug string) (bool, error) {
	if deployutils.IsNodeInlineAirplaneEntity(path) {
		if _, err := os.Stat(path); err != nil {
			return false, errors.Wrap(err, "opening file")
		}

		out, err := runNodeCommand(ctx, logger, javaScriptEntrypoint, "can_update_view", path, slug)
		if err != nil {
			return false, errors.WithMessagef(err, "checking if view can be updated at %q (re-run with --debug for more context)", path)
		}

		var canEdit bool
		if err := json.Unmarshal([]byte(out), &canEdit); err != nil {
			return false, errors.Wrap(err, "checking if view can be updated")
		}

		return canEdit, nil
	}

	return canUpdateYAMLView(path)
}

func updateYAMLView(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.ViewDefinition) error {
	format := definitions.GetViewDefFormat(path)
	if format == definitions.DefFormatUnknown {
		return errors.Errorf("updating views within %q files is not supported", filepath.Base(path))
	}

	content, err := def.Marshal(format)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return errors.Wrap(err, "opening definition file")
	}
	defer f.Close()

	if _, err := f.Write(content); err != nil {
		return errors.Wrap(err, "updating view")
	}

	return nil
}

func canUpdateYAMLView(path string) (bool, error) {
	if !definitions.IsViewDef(path) {
		return false, errors.Errorf("updating views within %q files is not supported", filepath.Base(path))
	}

	if _, err := os.Stat(path); err != nil {
		return false, errors.Wrap(err, "opening file")
	}

	return true, nil
}
