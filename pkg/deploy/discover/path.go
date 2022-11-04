package discover

import (
	"path/filepath"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/lib/pkg/utils/pathcase"
	"github.com/pkg/errors"
)

type TaskPathMetadata struct {
	AbsEntrypoint string
	RelEntrypoint string
	RootDir       string
	WorkDir       string
	BuildVersion  build.BuildTypeVersion
	BuildBase     build.BuildBase
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

	buildVersion, err := r.Version(taskroot)
	if err != nil {
		return TaskPathMetadata{}, err
	}

	var c config.AirplaneConfig
	configPath, found := fsx.Find(taskroot, config.FileName)
	if found {
		c, err = config.NewAirplaneConfigFromFile(filepath.Join(configPath, config.FileName))
		if err != nil {
			return TaskPathMetadata{}, err
		}
	}

	return TaskPathMetadata{
		AbsEntrypoint: absFile,
		RelEntrypoint: ep,
		RootDir:       taskroot,
		WorkDir:       wd,
		BuildVersion:  buildVersion,
		BuildBase:     c.Base,
	}, nil
}
