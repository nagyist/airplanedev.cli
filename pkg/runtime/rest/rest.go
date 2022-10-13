package rest

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/builtins"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/logger"
)

// Init register the runtime.
func init() {
	// will fallback to the task kind
	runtime.Register(".rest", Runtime{})
}

// Runtime implementation.
type Runtime struct{}

// PrepareRun implementation.
func (r Runtime) PrepareRun(ctx context.Context, logger logger.Logger, opts runtime.PrepareRunOptions) (rexprs []string, rcloser io.Closer, rerr error) {
	client, err := builtins.NewLocalClient(goruntime.GOOS, goruntime.GOARCH, logger)
	if err != nil {
		logger.Warning(err.Error())
		return nil, nil, err
	}
	req, err := builtins.MarshalRequest("airplane:rest_request", opts.KindOptions)
	if err != nil {
		return nil, nil, errors.New("invalid builtin request")
	}
	cmd, err := client.CmdString(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	return cmd, nil, nil
}

// Generate implementation.
func (r Runtime) Generate(t *runtime.Task) ([]byte, os.FileMode, error) {
	return nil, 0644, nil
}

// GenerateInline implementation.
func (r Runtime) GenerateInline(def *definitions.Definition_0_3) ([]byte, fs.FileMode, error) {
	return nil, 0, errors.New("cannot generate inline rest task configuration")
}

// Workdir implementation.
func (r Runtime) Workdir(path string) (string, error) {
	return r.Root(path)
}

// Root implementation.
func (r Runtime) Root(path string) (string, error) {
	return filepath.Dir(path), nil
}

// Kind implementation.
func (r Runtime) Kind() build.TaskKind {
	return build.TaskKindREST
}

// FormatComment implementation.
// REST does not have a file, so FormatComment does not apply
func (r Runtime) FormatComment(s string) string {
	return s
}

// SupportsLocalExecution implementation.
func (r Runtime) SupportsLocalExecution() bool {
	return true
}
