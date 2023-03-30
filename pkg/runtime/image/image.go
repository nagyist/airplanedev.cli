package image

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/runtime/updaters"
	"github.com/airplanedev/cli/pkg/utils/logger"
)

// Init register the runtime.
func init() {
	// will fallback to the task kind
	runtime.Register(".image", Runtime{})
}

// Runtime implementation.
type Runtime struct{}

// PrepareRun implementation.
func (r Runtime) PrepareRun(ctx context.Context, logger logger.Logger, opts runtime.PrepareRunOptions) (rexprs []string, rcloser io.Closer, rerr error) {
	return nil, nil, errors.New("cannot run docker image tasks")
}

// Generate implementation.
func (r Runtime) Generate(t *runtime.Task) ([]byte, os.FileMode, error) {
	return nil, 0644, nil
}

// GenerateInline implementation.
func (r Runtime) GenerateInline(def *definitions.Definition) ([]byte, fs.FileMode, error) {
	return nil, 0, errors.New("cannot generate inline docker image task configuration")
}

// Workdir implementation.
func (r Runtime) Workdir(path string) (string, error) {
	return r.Root(path)
}

// Root implementation.
func (r Runtime) Root(path string) (string, error) {
	return runtime.RootForNonBuiltRuntime(path)
}

func (r Runtime) Version(rootPath string) (buildVersion buildtypes.BuildTypeVersion, err error) {
	return "", nil
}

// Kind implementation.
func (r Runtime) Kind() buildtypes.TaskKind {
	return buildtypes.TaskKindImage
}

// FormatComment implementation.
// Image does not have a file, so FormatComment does not apply
func (r Runtime) FormatComment(s string) string {
	return s
}

// SupportsLocalExecution implementation.
func (r Runtime) SupportsLocalExecution() bool {
	return false
}

func (r Runtime) Update(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error {
	return updaters.UpdateYAML(ctx, logger, path, slug, def)
}

func (r Runtime) CanUpdate(ctx context.Context, logger logger.Logger, path string, slug string) (bool, error) {
	return updaters.CanUpdateYAML(path)
}
