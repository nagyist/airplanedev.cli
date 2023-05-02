package updaters

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/airplanedev/cli/pkg/definitions"
	deployutils "github.com/airplanedev/cli/pkg/deploy/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

//go:embed javascript/index.js
var javascriptUpdater []byte

var airplaneErrorRegex = regexp.MustCompile("__airplane_error (.*)\n")
var airplaneOutputRegex = regexp.MustCompile("__airplane_output (.*)\n")

func UpdateJavaScriptTask(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error {
	if deployutils.IsNodeInlineAirplaneEntity(path) {
		if _, err := os.Stat(path); err != nil {
			return errors.Wrap(err, "opening file")
		}

		defJSON, err := def.Marshal(definitions.DefFormatJSON)
		if err != nil {
			return errors.Wrap(err, "marshalling definition as JSON")
		}

		_, err = runNodeCommand(ctx, logger, javascriptUpdater, "update", path, slug, string(defJSON))
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

		out, err := runNodeCommand(ctx, logger, javascriptUpdater, "can_update", path, slug)
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

func runNodeCommand(ctx context.Context, logger logger.Logger, script []byte, args ...string) (string, error) {
	tempFile, err := os.CreateTemp("", "airplane.runtime.javascript-*")
	if err != nil {
		return "", errors.Wrap(err, "creating temporary file")
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.Write(script)
	if err != nil {
		return "", errors.Wrap(err, "writing script")
	}

	allArgs := append([]string{tempFile.Name()}, args...)
	cmd := exec.Command("node", allArgs...)
	logger.Debug("Running %s", strings.Join(cmd.Args, " "))

	out, err := cmd.Output()
	if len(out) == 0 {
		out = []byte("(none)")
	}
	logger.Debug("Output:\n%s", out)

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			matches := airplaneErrorRegex.FindStringSubmatch(string(ee.Stderr))
			if len(matches) >= 2 {
				errMsg := matches[1]
				return "", errors.New(errMsg)
			}
		}
		return "", errors.Wrap(err, "running node command")
	}

	matches := airplaneOutputRegex.FindStringSubmatch(string(out))
	if len(matches) >= 2 {
		msg := matches[1]
		return msg, nil
	}

	return "", nil
}
