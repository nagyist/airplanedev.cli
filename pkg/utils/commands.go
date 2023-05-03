package utils

import (
	"context"
	"os/exec"
	"strings"

	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

// GetPythonBinary returns the first of python3 or python found on PATH, if any.
//
// We expect most systems to have python3 if Python 3 is installed, as per PEP 0394:
// https://www.python.org/dev/peps/pep-0394/#recommendation
// However, Python on Windows (whether through Python or Anaconda) does not seem to install python3.exe.
func GetPythonBinary(ctx context.Context, logger logger.Logger) (string, error) {
	for _, bin := range []string{"python3", "python"} {
		logger.Debug("Looking for binary %q", bin)
		path, err := exec.LookPath(bin)
		if err == nil {
			logger.Debug("Found binary %q at %q", bin, path)

			// Confirm that this is a Python 3 binary.
			cmd := exec.CommandContext(ctx, bin, "--version")
			logger.Debug("Running %s", strings.Join(cmd.Args, " "))
			out, err := cmd.Output()
			if err != nil {
				logger.Debug("Got an error while running %q: %v", strings.Join(cmd.Args, " "), err)
			} else {
				version := string(out)
				if strings.HasPrefix(version, "Python 3.") {
					logger.Debug("Using %q with version %q", bin, strings.TrimSpace(version))
					return bin, nil
				}
				logger.Debug("Could not find Python 3 on your PATH. Found %q but running --version returned: %q", bin, version)
			}
		}
		logger.Debug("Could not find binary %q: %v", bin, err)
	}

	return "", errors.New("Could not find the python3 or python command on your PATH. Ensure that Python 3 is installed and available in your shell environment.")
}
