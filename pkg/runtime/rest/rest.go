package rest

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/builtins"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/runtime/transformers"
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
	if opts.BuiltinsClient == nil {
		return nil, nil, errors.New("builtins are not supported on this machine")
	}
	req, err := builtins.MarshalRequest("airplane:rest_request", opts.KindOptions)
	if err != nil {
		return nil, nil, errors.New("invalid builtin request")
	}
	cmd, err := opts.BuiltinsClient.CmdString(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	return cmd, opts.BuiltinsClient.Closer, nil
}

// Generate implementation.
func (r Runtime) Generate(t *runtime.Task) ([]byte, os.FileMode, error) {
	return nil, 0644, nil
}

// GenerateInline implementation.
func (r Runtime) GenerateInline(def *definitions.Definition) ([]byte, fs.FileMode, error) {
	return nil, 0, errors.New("cannot generate inline rest task configuration")
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
	return buildtypes.TaskKindREST
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

func (r Runtime) Edit(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error {
	return transformers.EditYAML(ctx, logger, path, slug, def)
}
