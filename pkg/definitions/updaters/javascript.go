package updaters

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"

	"github.com/airplanedev/cli/pkg/definitions"
	deployutils "github.com/airplanedev/cli/pkg/deploy/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

//go:embed javascript/index.js
var javaScriptEntrypoint []byte

func UpdateJavaScriptTask(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error {
	if deployutils.IsNodeInlineAirplaneEntity(path) {
		if _, err := os.Stat(path); err != nil {
			return errors.Wrap(err, "opening file")
		}

		defJSON, err := def.Marshal(definitions.DefFormatJSON)
		if err != nil {
			return errors.Wrap(err, "marshalling definition as JSON")
		}

		_, err = runNodeCommand(ctx, logger, javaScriptEntrypoint, "update", path, slug, string(defJSON))
		if err != nil {
			return errors.WithMessagef(err, "updating task at %q (re-run with --debug for more context)", path)
		}

		return nil
	}

	return UpdateYAMLTask(ctx, logger, path, slug, def)
}

func CanUpdateJavaScriptTask(ctx context.Context, logger logger.Logger, path string, slug string) (bool, error) {
	if deployutils.IsNodeInlineAirplaneEntity(path) {
		if _, err := os.Stat(path); err != nil {
			return false, errors.Wrap(err, "opening file")
		}

		out, err := runNodeCommand(ctx, logger, javaScriptEntrypoint, "can_update", path, slug)
		if err != nil {
			return false, errors.WithMessagef(err, "checking if task can be updated at %q (re-run with --debug for more context)", path)
		}

		var canEdit bool
		if err := json.Unmarshal([]byte(out), &canEdit); err != nil {
			return false, errors.Wrap(err, "checking if task can be updated")
		}

		return canEdit, nil
	}

	return CanUpdateYAMLTask(path)
}
