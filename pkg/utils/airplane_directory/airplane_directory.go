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
		// Only set a closer if the current run caused the creation of the .airplane/task directory; else,
		// defer removal of .airplane/task to the parent run.
		// TODO: let the dev server clean up deps when it shuts down
		closer = CloseFunc(func() error {
			return errors.Wrap(os.RemoveAll(taskDir), "unable to remove temporary task directory")
		})
	} else if !os.IsExist(err) {
		// Don't error if a task-specific subdirectory already exists. We don't clear the subdirectory's contents here
		// since this could cause unknown behavior if multiple runs of a given task are occurring in parallel. Instead,
		// we rely on the runtime-specific implementation of PrepareRun to overwrite the existing shim, etc.
		return "", "", nil, errors.Wrap(err, "creating task subdirectory in .airplane")

	}

	return airplaneDir, taskDir, closer, nil
}
