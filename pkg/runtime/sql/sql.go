package sql

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/builtins"
	"github.com/airplanedev/cli/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/runtime/updaters"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
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

type Runtime struct{}

func (r Runtime) PrepareRun(ctx context.Context, logger logger.Logger, opts runtime.PrepareRunOptions) (rexprs []string, rcloser io.Closer, rerr error) {
	if opts.BuiltinsClient == nil {
		return nil, nil, errors.New("builtins are not supported on this machine")
	}
	req, err := builtins.MarshalRequest("airplane:sql_query", opts.KindOptions)
	if err != nil {
		return nil, nil, errors.New("invalid builtin request")
	}
	cmd, err := opts.BuiltinsClient.CmdString(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	return cmd, opts.BuiltinsClient.Closer, nil
}

func (r Runtime) Generate(t *runtime.Task) ([]byte, os.FileMode, error) {
	return code, 0644, nil
}

func (r Runtime) GenerateInline(def *definitions.Definition) ([]byte, fs.FileMode, error) {
	return nil, 0, errors.New("cannot generate inline sql task configuration")
}

func (r Runtime) Workdir(path string) (string, error) {
	return r.Root(path)
}

func (r Runtime) Root(path string) (string, error) {
	return runtime.RootForNonBuiltRuntime(path)
}

func (r Runtime) Version(rootPath string) (buildVersion buildtypes.BuildTypeVersion, err error) {
	return "", nil
}

func (r Runtime) Kind() buildtypes.TaskKind {
	return buildtypes.TaskKindSQL
}

func (r Runtime) FormatComment(s string) string {
	var lines []string

	for _, line := range strings.Split(s, "\n") {
		lines = append(lines, "-- "+line)
	}

	return strings.Join(lines, "\n")
}

func (r Runtime) SupportsLocalExecution() bool {
	return true
}

func (r Runtime) Update(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error {
	if err := updaters.UpdateYAML(ctx, logger, path, slug, def); err != nil {
		return err
	}

	if def.SQL == nil {
		return errors.New("SQL configuration missing on definition")
	}

	query, err := def.SQL.GetQuery()
	if err != nil {
		return err
	}

	entrypoint := filepath.Join(filepath.Dir(def.GetDefnFilePath()), def.SQL.Entrypoint)
	f, err := os.OpenFile(entrypoint, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return errors.Wrap(err, "opening entrypoint")
	}
	defer f.Close()

	if _, err := f.Write([]byte(query)); err != nil {
		return errors.Wrap(err, "updating entrypoint")
	}

	return nil
}

func (r Runtime) CanUpdate(ctx context.Context, logger logger.Logger, path string, slug string) (bool, error) {
	if canUpdate, err := updaters.CanUpdateYAML(path); err != nil {
		return false, err
	} else if !canUpdate {
		return false, nil
	}

	if _, err := os.Stat(path); err != nil {
		return false, errors.Wrap(err, "opening file")
	}

	return true, nil
}
