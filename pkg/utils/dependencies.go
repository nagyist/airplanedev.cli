package utils

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/lib/pkg/utils/fsx"
)

func InstallDependencies(dir string, useYarn bool) error {
	var cmd *exec.Cmd
	if useYarn {
		cmd = exec.Command("yarn")
		logger.Debug("Installing dependencies with yarn")
	} else {
		cmd = exec.Command("npm", "install")
		logger.Debug("Installing dependencies with npm")
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
			logger.Debug(m)
		}
	}()
	if err = cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
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
