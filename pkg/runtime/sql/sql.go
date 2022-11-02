package sql

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/builtins"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/logger"
)

// Init register the runtime.
func init() {
	runtime.Register(".sql", Runtime{})
}

// Code.
var code = []byte(`-- Add your SQL queries here.
-- See SQL documentation: https://docs.airplane.dev/creating-tasks/sql
SELECT 1;
`)

// Runtime implementation.
type Runtime struct{}

// PrepareRun implementation.
func (r Runtime) PrepareRun(ctx context.Context, logger logger.Logger, opts runtime.PrepareRunOptions) (rexprs []string, rcloser io.Closer, rerr error) {
	builtinClient, err := builtins.NewLocalClient(goruntime.GOOS, goruntime.GOARCH, logger)
	if err != nil {
		logger.Warning(err.Error())
		return nil, nil, err
	}

	// Reload the sql entrypoint here. This supports changing the sql file without
	// hot reloading the definition.
	query, err := os.ReadFile(opts.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read sql file %s: %v", opts.Path, err)
	}
	opts.KindOptions["query"] = string(query)

	req, err := builtins.MarshalRequest("airplane:sql_query", opts.KindOptions)
	if err != nil {
		return nil, nil, errors.New("invalid builtin request")
	}
	cmd, err := builtinClient.CmdString(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	return cmd, nil, nil
}

// Generate implementation.
func (r Runtime) Generate(t *runtime.Task) ([]byte, os.FileMode, error) {
	return code, 0644, nil
}

// GenerateInline implementation.
func (r Runtime) GenerateInline(def *definitions.Definition_0_3) ([]byte, fs.FileMode, error) {
	return nil, 0, errors.New("cannot generate inline sql task configuration")
}

// Workdir implementation.
func (r Runtime) Workdir(path string) (string, error) {
	return r.Root(path)
}

// Root implementation.
func (r Runtime) Root(path string) (string, error) {
	return filepath.Dir(path), nil
}

func (r Runtime) Version(rootPath string) (buildVersion build.BuildTypeVersion, err error) {
	return "", nil
}

// Kind implementation.
func (r Runtime) Kind() build.TaskKind {
	return build.TaskKindSQL
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
