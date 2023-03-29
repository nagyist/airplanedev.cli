package updaters

import (
	"context"
	"os"
	"path/filepath"

	"github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

func UpdateYAML(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error {
	format := definitions.GetTaskDefFormat(path)
	if format == definitions.DefFormatUnknown {
		return errors.Errorf("updating tasks within %q files is not supported", filepath.Base(path))
	}

	// Apply a default value to the timeout field.
	if def.Timeout == 3600 && def.Runtime == types.TaskRuntimeStandard {
		def.Timeout = 0
	}

	content, err := def.Marshal(format)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return errors.Wrap(err, "opening definition file")
	}
	defer f.Close()

	if _, err := f.Write(content); err != nil {
		return errors.Wrap(err, "updating task")
	}

	return nil
}

func CanUpdateYAML(path string) (bool, error) {
	if !definitions.IsTaskDef(path) {
		return false, errors.Errorf("updating tasks within %q files is not supported", filepath.Base(path))
	}

	if _, err := os.Stat(path); err != nil {
		return false, errors.Wrap(err, "opening file")
	}

	return true, nil
}
