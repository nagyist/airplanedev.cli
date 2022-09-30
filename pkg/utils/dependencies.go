package utils

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/blang/semver"
	"github.com/pkg/errors"
)

const requiredNodeVersion = "14.18.0"

func InstallDependencies(dir string, useYarn bool) error {
	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{WithLoader: true})
	defer l.StopLoader()

	var cmd *exec.Cmd
	if useYarn {
		cmd = exec.Command("yarn")
		l.Debug("Installing dependencies with yarn")
	} else {
		cmd = exec.Command("npm", "install")
		l.Debug("Installing dependencies with npm")
	}
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	scanner := bufio.NewScanner(stdout)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for scanner.Scan() {
			m := scanner.Text()
			l.Debug(m)
		}
	}()
	if err = cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}

func CheckNodeVersion() error {
	out, err := exec.Command("node", "-v").Output()
	if err != nil {
		return err
	}
	if len(out) == 0 || out[0] != 'v' {
		return errors.New("Invalid nodejs version: " + string(out))
	}

	version, err := semver.Make(strings.Trim(string(out[1:]), "\n"))
	if err != nil {
		return err
	}
	reqVersion, err := semver.Make(requiredNodeVersion)
	if err != nil {
		return err
	}
	if version.Compare(reqVersion) < 0 {
		return errors.New("Requires nodejs version >= " + requiredNodeVersion)
	}
	return nil
}

func ShouldUseYarn(packageJSONDirPath string) bool {
	// If the closest directory with a package.json has a lockfile, we will use that to
	// determine whether to use yarn or npm even if we eventually create a new package.json for the view.
	yarnlock := filepath.Join(packageJSONDirPath, "yarn.lock")
	pkglock := filepath.Join(packageJSONDirPath, "package-lock.json")

	if fsx.Exists(yarnlock) {
		logger.Debug("Using yarn to manage dependencies because of yarn.lock in parent directory")
		return true
	} else if fsx.Exists(pkglock) {
		logger.Debug("Using npm to manage dependencies because of package-lock.json in parent directory")
		return false
	}

	// No lockfiles, so check if yarn is installed by getting yarn version
	cmd := exec.Command("yarn", "-v")
	cmd.Dir = filepath.Dir(packageJSONDirPath)
	err := cmd.Run()
	return err == nil
}
