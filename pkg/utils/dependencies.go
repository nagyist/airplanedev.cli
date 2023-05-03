package utils

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/blang/semver"
	"github.com/pkg/errors"
)

const (
	requiredNodeVersion = "14.18.0"
	SymlinkErrString    = "Error: EPERM: operation not permitted, symlink"
)

type InstallOptions struct {
	Yarn       bool
	NoBinLinks bool
}

func InstallDependencies(dir string, opts InstallOptions) error {
	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{WithLoader: true})
	defer l.StopLoader()

	args := []string{"install"}
	if opts.NoBinLinks {
		args = append(args, "--no-bin-links")
	}
	var cmd *exec.Cmd
	if opts.Yarn {
		cmd = exec.Command("yarn", args...)
		l.Debug("Installing dependencies with yarn")
	} else {
		cmd = exec.Command("npm", args...)
		l.Debug("Installing dependencies with npm")
	}
	if opts.NoBinLinks {
		l.Debug("Installing with --no-bin-links")
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
	logsMutex := &sync.Mutex{}
	var allLogs []string
	go func() {
		defer wg.Done()
		for scanner.Scan() {
			m := scanner.Text()
			l.Debug(m)
			logsMutex.Lock()
			allLogs = append(allLogs, m)
			logsMutex.Unlock()
		}
	}()
	if err = cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		errString := strings.Join(allLogs, "\n")
		if !opts.NoBinLinks && strings.Contains(errString, SymlinkErrString) {
			// Try installation again with NoBinLinks to get passed the symlink error.
			opts.NoBinLinks = true
			return InstallDependencies(dir, opts)
		}
		return errors.New(errString)
	}
	return nil
}

func CheckNodeVersion() error {
	out, err := exec.Command("node", "-v").Output()
	if err != nil {
		return err
	}
	if len(out) == 0 || out[0] != 'v' {
		return errors.New("Invalid nodejs version: " + string(out))
	}

	version, err := semver.Make(strings.TrimSpace(string(out[1:])))
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
