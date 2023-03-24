package transformers

import (
	"context"
	"os"
	"path/filepath"

	"github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

func EditYAML(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error {
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
	if err := os.WriteFile(path, content, 0655); err != nil {
		return errors.Wrap(err, "editing task")
	}
	return nil
}
