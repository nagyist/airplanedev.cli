package taskdir

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/pkg/errors"
)

func (td TaskDirectory) ReadDefinition() (definitions.Definition, error) {
	buf, err := os.ReadFile(td.defPath)
	if err != nil {
		return definitions.Definition{}, errors.Wrap(err, "reading task definition")
	}

	defPath := td.defPath
	// Attempt to set a prettier defPath, best effort
	if wd, err := os.Getwd(); err != nil {
	} else if path, err := filepath.Rel(wd, defPath); err != nil {
	} else {
		defPath = path
	}

	def := definitions.Definition{}
	if err := def.Unmarshal(definitions.GetTaskDefFormat(defPath), buf); err != nil {
		switch err := errors.Cause(err).(type) {
		case definitions.ErrSchemaValidation:
			errorMsgs := []string{}
			for _, verr := range err.Errors {
				errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", verr.Field(), verr.Description()))
			}
			return definitions.Definition{}, definitions.NewErrReadDefinition(fmt.Sprintf("Error reading %s", defPath), errorMsgs...)
		default:
			return definitions.Definition{}, errors.Wrap(err, "unmarshalling task definition")
		}
	}
	def.SetDefnFilePath(td.defPath)
	entrypoint, err := def.Entrypoint()
	if err == definitions.ErrNoEntrypoint {
		// nothing
	} else if err != nil {
		return definitions.Definition{}, err
	} else {
		if filepath.IsAbs(entrypoint) {
			if err := def.SetAbsoluteEntrypoint(entrypoint); err != nil {
				return definitions.Definition{}, err
			}
		} else {
			defnDir := filepath.Dir(td.defPath)
			absEntrypoint, err := filepath.Abs(filepath.Join(defnDir, entrypoint))
			if err != nil {
				return definitions.Definition{}, err
			}
			if err := def.SetAbsoluteEntrypoint(absEntrypoint); err != nil {
				return definitions.Definition{}, err
			}
		}
	}
	return def, nil
}
