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

// CreateTaskDir creates an ephemeral task directory for local execution of a task, creating/cleaning up a top-level
// .airplane directory as needed.
func CreateTaskDir(root string, slug string) (airplaneDir string, taskDir string, closer io.Closer, err error) {
	closer = CloseFunc(func() error { return nil })

	// Create a .airplane directory in the root of the task, if it doesn't already exist.
	airplaneDir = filepath.Join(root, ".airplane")
	if err := os.Mkdir(airplaneDir, os.ModeDir|0777); err == nil {
		// Only set a closer if the current run caused the creation of the .airplane directory; else, defer removal of
		// .airplane/ to the parent run.
		closer = CloseFunc(func() error {
			return errors.Wrap(os.RemoveAll(airplaneDir), "unable to remove temporary directory")
		})
	} else if !os.IsExist(err) {
		return "", "", nil, errors.Wrap(err, "creating .airplane directory")
	}

	// Create a .airplane/{task_slug}/ subdirectory for this task.
	taskDir = filepath.Join(airplaneDir, slug)
	if err := os.Mkdir(taskDir, os.ModeDir|0777); err != nil {
		// Don't error if a task-specific subdirectory already exists. We don't clear the subdirectory's contents here
		// since this could cause unknown behavior if multiple runs of a given task are occurring in parallel. Instead,
		// we rely on the runtime-specific implementation of PrepareRun to overwrite the existing shim, etc.
		if !os.IsExist(err) {
			return "", "", closer, errors.Wrap(err, "creating task subdirectory")
		}
	}

	return airplaneDir, taskDir, closer, nil
}
