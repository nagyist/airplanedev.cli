package discover

import (
	"path/filepath"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/pathcase"
	"github.com/pkg/errors"
)

type TaskPathMetadata struct {
	AbsEntrypoint string
	RelEntrypoint string
	RootDir       string
	WorkDir       string
	Runtime       runtime.Interface
}

func taskPathMetadata(file string, kind build.TaskKind) (TaskPathMetadata, error) {
	r, err := runtime.Lookup(file, kind)
	if err != nil {
		return TaskPathMetadata{}, errors.Wrapf(err, "cannot determine how to deploy %q - check your CLI is up to date", file)
	}

	absFile, err := filepath.Abs(file)
	if err != nil {
		return TaskPathMetadata{}, err
	}

	taskroot, err := r.Root(absFile)
	if err != nil {
		return TaskPathMetadata{}, err
	}

	wd, err := r.Workdir(absFile)
	if err != nil {
		return TaskPathMetadata{}, err
	}

	// Entrypoint needs to be relative to the taskroot.
	absEntrypoint, err := pathcase.ActualFilename(absFile)
	if err != nil {
		return TaskPathMetadata{}, err
	}
	ep, err := filepath.Rel(taskroot, absEntrypoint)
	if err != nil {
		return TaskPathMetadata{}, err
	}

	return TaskPathMetadata{
		AbsEntrypoint: absFile,
		RelEntrypoint: ep,
		RootDir:       taskroot,
		WorkDir:       wd,
		Runtime:       r,
	}, nil
}
