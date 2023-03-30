package builtin

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/builtins"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/runtime/updaters"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

// Init register the runtime.
func init() {
	runtime.Register(".builtin", Runtime{})
}

// Runtime implementation.
type Runtime struct{}

// PrepareRun implementation.
func (r Runtime) PrepareRun(ctx context.Context, logger logger.Logger, opts runtime.PrepareRunOptions) (rexprs []string, rcloser io.Closer, rerr error) {
	if opts.BuiltinsClient == nil {
		return nil, nil, errors.New("builtins are not supported on this machine")
	}
	fs, err := builtins.GetFunctionSpecificationFromKindOptions(opts.KindOptions)
	if err != nil {
		return nil, nil, err
	}
	request, ok := opts.KindOptions["request"]
	if !ok {
		return nil, nil, errors.New("missing request from builtin KindOptions")
	}
	requestMap, ok := request.(map[string]interface{})
	if !ok {
		return nil, nil, errors.Errorf("expected map request, got %T instead", request)
	}
	req, err := builtins.MarshalRequest(
		fmt.Sprintf("airplane:%s_%s", fs.Namespace, fs.Name),
		requestMap,
	)
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
	return nil, 0, errors.New("cannot generate inline builtin task configuration")
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
	return buildtypes.TaskKindBuiltin
}

// FormatComment implementation.
func (r Runtime) FormatComment(s string) string {
	var lines []string

	for _, line := range strings.Split(s, "\n") {
		lines = append(lines, "-- "+line)
	}

	return strings.Join(lines, "\n")
}

// SupportsLocalExecution implementation.
func (r Runtime) SupportsLocalExecution() bool {
	return true
}

func (r Runtime) Update(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error {
	return updaters.UpdateYAML(ctx, logger, path, slug, def)
}

func (r Runtime) CanUpdate(ctx context.Context, logger logger.Logger, path string, slug string) (bool, error) {
	return updaters.CanUpdateYAML(path)
}
