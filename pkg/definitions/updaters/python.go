package updaters

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/deploy/config"
	deployutils "github.com/airplanedev/cli/pkg/deploy/utils"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

//go:embed python/updater.pex
var pythonEntrypoint []byte

var testingOnlyIgnoreCurrentPythonVersion bool

func UpdatePythonTask(ctx context.Context, logger logger.Logger, root, path string, slug string, def definitions.Definition) error {
	if deployutils.IsPythonInlineAirplaneEntity(path) {
		if utils.IsWindows() {
			return errors.New("Updating inline Python tasks is not supported on Windows.")
		}

		if _, err := os.Stat(path); err != nil {
			return errors.Wrap(err, "opening file")
		}

		defJSON, err := def.Marshal(definitions.DefFormatJSON)
		if err != nil {
			return errors.Wrap(err, "marshalling definition as JSON")
		}

		pyVersion := ""
		if config.HasAirplaneConfig(root) {
			cfg, err := config.NewAirplaneConfigFromFile(root)
			if err != nil {
				return errors.Wrap(err, "opening configuration file")
			}
			pyVersion = cfg.Python.Version
		}
		envVars := []string{
			// Store the pex venv under the .airplane directory instead of the user's home directory.
			"PEX_ROOT=" + filepath.Join(conf.Dir(), "pex"),
		}
		if testingOnlyIgnoreCurrentPythonVersion {
			envVars = append(envVars, "TESTING_ONLY_IGNORE_CURRENT_PYTHON_VERSION=true")
		}

		bin, err := utils.GetPythonBinary(ctx, logger)
		if err != nil {
			return err
		}

		_, err = runPythonCommand(ctx, logger, pythonEntrypoint, runCommandRequest{
			Command: bin,
			Args:    []string{"update", path, slug, string(defJSON), pyVersion},
			EnvVars: envVars,
		})
		if err != nil {
			return errors.WithMessagef(err, "updating task at %q (re-run with --debug for more context)", path)
		}

		return nil
	}

	return UpdateYAMLTask(ctx, logger, path, slug, def)
}

func CanUpdatePythonTask(ctx context.Context, logger logger.Logger, path string, slug string) (bool, error) {
	if deployutils.IsPythonInlineAirplaneEntity(path) {
		if utils.IsWindows() {
			return false, nil
		}

		if _, err := os.Stat(path); err != nil {
			return false, errors.Wrap(err, "opening file")
		}

		bin, err := utils.GetPythonBinary(ctx, logger)
		if err != nil {
			return false, err
		}

		out, err := runPythonCommand(ctx, logger, pythonEntrypoint, runCommandRequest{
			Command: bin,
			Args:    []string{"can_update", path, slug},
			EnvVars: []string{
				// Store the pex venv under the .airplane directory instead of the user's home directory.
				"PEX_ROOT=" + filepath.Join(conf.Dir(), "pex"),
			},
		})
		if err != nil {
			return false, errors.WithMessagef(err, "checking if task can be updated at %q (re-run with --debug for more context)", path)
		}

		var canEdit bool
		if out != "" {
			if err := json.Unmarshal([]byte(out), &canEdit); err != nil {
				return false, errors.Wrap(err, "checking if task can be updated")
			}
		}

		return canEdit, nil
	}

	return CanUpdateYAMLTask(path)
}
