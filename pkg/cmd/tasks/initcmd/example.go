package initcmd

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/taskdir"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

func initWithExample(ctx context.Context, cfg config) error {
	dir, err := taskdir.Open(cfg.from)
	if err != nil {
		return err
	}
	defer dir.Close()

	def, err := dir.ReadDefinition()
	if err != nil {
		return err
	}

	kind, _, err := def.GetKindAndOptions()
	if err != nil {
		return err
	}

	// Select the dst directory. Default to the definition slug. If you pass us a folder/file, try
	// to pick the nearest directory.
	dstDirectory := def.GetSlug()
	if cfg.file != "" {
		fi, err := os.Stat(cfg.file)
		if os.IsNotExist(err) {
			// File doesn't exist. If it has a dot, assume it's a file; otherwise, assume it's a
			// directory.
			if filepath.Ext(cfg.file) == "" {
				dstDirectory = cfg.file
			} else {
				dstDirectory = filepath.Dir(cfg.file)
			}
		} else if err != nil {
			return err
		} else {
			// File exists. Pick the nearest directory.
			if fi.IsDir() {
				dstDirectory = cfg.file
			} else {
				dstDirectory = filepath.Dir(cfg.file)
			}
		}
	}

	defPath := dir.DefinitionPath()
	localDefPath := ""
	entrypoint, err := def.GetAbsoluteEntrypoint()
	localExecutionSupported := false
	if err == definitions.ErrNoEntrypoint {
		// No entrypoint: just copy the definition file.
		localDefPath = filepath.Join(dstDirectory, filepath.Base(defPath))
		if fsx.Exists(dstDirectory) {
			// Warn if we're going to overwrite something.
			if fsx.Exists(localDefPath) {
				question := fmt.Sprintf("File %s exists. Would you like to overwrite it?", localDefPath)
				if ok, err := utils.ConfirmWithAssumptions(question, cfg.assumeYes, cfg.assumeNo); err != nil {
					return err
				} else if !ok {
					// bail here, nothing to do.
					logger.Step("Initialization aborted.")
					return nil
				}
			}
		} else {
			if err := os.MkdirAll(dstDirectory, 0755); err != nil {
				return err
			}
			logger.Step("Created folder %s", dstDirectory)
		}

		if err := copyFile(defPath, dstDirectory); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		// Entrypoint exists: copy the whole root into a new folder.
		r, err := runtime.Lookup(entrypoint, kind)
		if err != nil {
			return err
		}
		localExecutionSupported = r.SupportsLocalExecution()

		taskroot, err := r.Root(entrypoint)
		if err != nil {
			return err
		}

		if fsx.Exists(dstDirectory) {
			// Warn if we're going to overwrite something.
			if err := checkForFileCollisions(taskroot, dstDirectory); err == ErrFileCollisionsExist {
				question := fmt.Sprintf("Folder %s exists. Continue anyway? (Some files may get overwritten.)", dstDirectory)
				if ok, err := utils.ConfirmWithAssumptions(question, cfg.assumeYes, cfg.assumeNo); err != nil {
					return err
				} else if !ok {
					// bail here, nothing to do.
					logger.Step("Initialization aborted.")
					return nil
				}
			} else if err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(dstDirectory, 0755); err != nil {
				return err
			}
			logger.Step("Created folder %s", dstDirectory)
		}

		if err := copyDirectoryContents(taskroot, dstDirectory); err != nil {
			return err
		}

		rel, err := filepath.Rel(taskroot, defPath)
		if err != nil {
			return err
		}
		localDefPath = filepath.Join(dstDirectory, rel)
	}

	suggestNextSteps(suggestNextStepsRequest{
		defnFile:           localDefPath,
		entrypoint:         entrypoint,
		showLocalExecution: localExecutionSupported,
		kind:               kind,
	})
	return nil
}

func copyFile(srcPath string, dstDirectory string) error {
	src, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return errors.Wrapf(err, "reading src file %s", srcPath)
	}

	dstPath := filepath.Join(dstDirectory, filepath.Base(srcPath))
	if err := ioutil.WriteFile(dstPath, src, 0644); err != nil {
		return errors.Wrapf(err, "writing dst file %s", dstPath)
	}
	logger.Step("Copied %s", dstPath)
	return nil
}

func copyDirectoryContents(srcDirectory string, dstDirectory string) error {
	return filepath.WalkDir(srcDirectory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == srcDirectory {
			return nil
		}

		rel, err := filepath.Rel(srcDirectory, path)
		if err != nil {
			return err
		}

		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dstDirectory, rel), 0755)
		}
		return copyFile(path, filepath.Join(dstDirectory, filepath.Dir(rel)))
	})
}

var ErrFileCollisionsExist = errors.New("File collisions exist")

func checkForFileCollisions(srcDirectory string, dstDirectory string) error {
	return filepath.WalkDir(srcDirectory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == srcDirectory {
			return nil
		}

		rel, err := filepath.Rel(srcDirectory, path)
		if err != nil {
			return err
		}

		if fsx.Exists(filepath.Join(dstDirectory, rel)) {
			return ErrFileCollisionsExist
		}
		return nil
	})
}
