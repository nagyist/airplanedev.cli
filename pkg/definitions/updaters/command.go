package updaters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

var (
	airplaneErrorRegex  = regexp.MustCompile(`__airplane_error (.*)\n`)
	airplaneOutputRegex = regexp.MustCompile(`__airplane_output (.*)\n`)
)

func runNodeCommand(ctx context.Context, logger logger.Logger, script []byte, args ...string) (string, error) {
	tempFile, err := os.CreateTemp("", "airplane.runtime.javascript-*.js")
	if err != nil {
		return "", errors.Wrap(err, "creating temporary file")
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.Write(script)
	if err != nil {
		return "", errors.Wrap(err, "writing script")
	}

	return runCommand(ctx, logger, runCommandRequest{
		Command: "node",
		Args:    append([]string{tempFile.Name()}, args...),
	})
}

func runPythonCommand(ctx context.Context, logger logger.Logger, script []byte, req runCommandRequest) (string, error) {
	tempFile, err := os.CreateTemp("", "airplane_runtime_python_*.py")
	if err != nil {
		return "", errors.Wrap(err, "creating temporary file")
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.Write(script)
	if err != nil {
		return "", errors.Wrap(err, "writing script")
	}

	return runCommand(ctx, logger, runCommandRequest{
		Command: req.Command,
		// Set `-s` to keep sys.path clean (https://docs.python.org/3/using/cmdline.html#cmdoption-s)
		Args:    append([]string{"-s", tempFile.Name()}, req.Args...),
		EnvVars: req.EnvVars,
	})
}

type runCommandRequest struct {
	Command string
	Args    []string
	EnvVars []string
}

func runCommand(ctx context.Context, logger logger.Logger, req runCommandRequest) (string, error) {
	cmd := exec.Command(req.Command, req.Args...)
	withStr := ""
	if len(req.EnvVars) > 0 {
		cmd.Env = append(os.Environ(), req.EnvVars...)
		withStr = fmt.Sprintf(" with env: %v", req.EnvVars)
	}
	logger.Debug("Running %q%s", strings.Join(cmd.Args, " "), withStr)

	out, err := cmd.CombinedOutput()
	if len(out) == 0 {
		out = []byte("(none)")
	}
	logger.Debug("Output:\n%s", out)

	if err != nil {
		matches := airplaneErrorRegex.FindSubmatch(out)
		if len(matches) >= 2 {
			rawErr := matches[1]
			errMsg := ""
			if err := json.Unmarshal(rawErr, &errMsg); err != nil {
				logger.Debug("Unable to unmarshal error message: %+v", err)
			} else {
				return "", errors.New(errMsg)
			}
		} else {
			logger.Debug("got exit error but could not match...")
		}
		return "", errors.Wrap(err, "running command")
	}

	matches := airplaneOutputRegex.FindStringSubmatch(string(out))
	if len(matches) >= 2 {
		msg := matches[1]
		return msg, nil
	}

	return "", nil
}
