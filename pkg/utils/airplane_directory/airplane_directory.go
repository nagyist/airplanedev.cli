package airplane_directory

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func CloseFunc(f func() error) io.Closer {
	return closer{f: f}
}

type closer struct {
	f func() error
}

func (c closer) Close() error {
	return c.f()
}

func CreateAirplaneDir(root string) (string, error) {
	// Create a .airplane directory in the root of the task, if it doesn't already exist.
	airplaneDir := filepath.Join(root, ".airplane")
	if err := os.Mkdir(airplaneDir, os.ModeDir|0777); err != nil && !os.IsExist(err) {
		return "", errors.Wrap(err, "creating .airplane directory")
	}
	return airplaneDir, nil
}

// CreateTaskDir creates an ephemeral .airplane directory if it doesn't exist, and
// a nested task directory for local execution of a task inside it.
func CreateTaskDir(root string, slug string) (string, string, io.Closer, error) {
	closer := CloseFunc(func() error { return nil })

	airplaneDir, err := CreateAirplaneDir(root)
	if err != nil {
		return "", "", nil, err
	}

	// Create a .airplane/{task_slug}/ subdirectory for this task.
	taskDir := filepath.Join(airplaneDir, slug)
	if err := os.Mkdir(taskDir, os.ModeDir|0777); err == nil {
		// TODO: Don't return a closer. The javascript runtime does not utilize a closer, but the other runtimes do.
		// We should keep task directories around for all task kinds.
		closer = CloseFunc(func() error {
			return errors.Wrap(os.RemoveAll(taskDir), "unable to remove task directory")
		})
	} else if !os.IsExist(err) {
		// Don't error if a task-specific subdirectory already exists.
		return "", "", nil, errors.Wrap(err, "creating task subdirectory in .airplane")
	}

	return airplaneDir, taskDir, closer, nil
}

// CreateRunDir creates a nested run directory inside a task directory for the duration of a local run.
func CreateRunDir(taskDir string, runID string) (string, io.Closer, error) {
	closer := CloseFunc(func() error { return nil })

	// Create a .airplane/{task_slug}/{run_id} subdirectory for this run.
	runDir := filepath.Join(taskDir, runID)
	if err := os.Mkdir(runDir, os.ModeDir|0777); err == nil {
		closer = CloseFunc(func() error {
			return errors.Wrap(os.RemoveAll(runDir), "unable to remove temporary run directory")
		})
	} else if !os.IsExist(err) {
		// Don't error if a task-specific subdirectory already exists. We don't clear the subdirectory's contents here
		// since this could cause unknown behavior if multiple runs of a given task are occurring in parallel. Instead,
		// we rely on the runtime-specific implementation of PrepareRun to overwrite the existing shim, etc.
		return "", nil, errors.Wrap(err, "creating run subdirectory in .airplane")
	}

	return runDir, closer, nil
}
