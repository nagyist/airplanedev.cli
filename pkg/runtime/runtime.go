// Package runtime generates code to match a runtime.
//
// The runtime package is capable of writing airplane specific
// comments that are used to link a task file to a remote task.
//
// All runtimes are also capable of generating initial code to
// match the task, including the parameters.
package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/builtins"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/deploy/config"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

var (
	// ErrMissing is returned when a resource was not found.
	//
	// It can be checked via `errors.Is(err, ErrMissing)`.
	ErrMissing = errors.New("runtime: resource is missing")

	// ErrNotImplemented is returned when a runtime does not support a runtime method.
	//
	// It can be checked via `errors.Is(err, ErrNotImplemented)`.
	ErrNotImplemented = errors.New("runtime: not implemented")
)

// Interface represents a runtime.
type Interface interface {
	// Generate accepts a task and generates code to match the task.
	//
	// os.FileMode is used for the permissions of the generated file. Files will typically use 0644
	// but might use 0744 for executable scripts (e.g. shell scripts).
	Generate(task *Task) ([]byte, os.FileMode, error)

	// GenerateInline accepts a definition and generates code to match
	// the task.
	//
	// os.FileMode is used for the permissions of the generated file. Files will typically use 0644
	// but might use 0744 for executable scripts (e.g. shell scripts).
	GenerateInline(def *definitions.Definition) ([]byte, os.FileMode, error)

	// Workdir attempts to detect the root of the given task path.
	//
	// Unlike root it decides the dockerfile's `workdir` directive
	// this might be different than root because it decides where
	// the build commands are run.
	Workdir(path string) (dir string, err error)

	// Root attempts to detect the root of the given task path.
	//
	// Typically runtimes will look for a specific file such as
	// `package.json` or `requirements.txt`, they'll use `fs.Find()`.
	Root(path string) (dir string, err error)

	// Version attempts to detect the version that the entity should be built with. Returns
	// an empty version if it cannot be determined.
	//
	// Typically runtimes will look for the version in a specific file such as `package.json`.
	Version(rootPath string) (buildVersion buildtypes.BuildTypeVersion, err error)

	// Kind returns a task kind that matches the runtime.
	//
	// Generate and other methods should not be called
	// for a task that doesn't match the returned kind.
	Kind() buildtypes.TaskKind

	// FormatComment formats a string into a comment using
	// the relevant comment characters for this runtime.
	FormatComment(s string) string

	// PrepareRun should prepare a local run of a task.
	//
	// It must create a temporary directory, install any dependencies
	// and prepare the script to be run.
	//
	// On success the method returns a slice that represents an `cmd.Exec`
	// options which contains the command to be run and its arguments. It
	// should also return an io.Closer that will be called when the local
	// run has completed. If not needed, a nil closer can be returned. If
	// an error is encountered during PrepareRun, the runtime should perform
	// its own cleanup.
	//
	// If running the script locally is not supported the method returns
	// an `ErrNotImplemented`.
	PrepareRun(ctx context.Context, logger logger.Logger, opts PrepareRunOptions) (rexprs []string, closer io.Closer, err error)

	// SupportsLocalExecution returns true if local execution is supported.
	// This is expected to match whether PrepareRun returns `ErrNotImplemented`.
	SupportsLocalExecution() bool

	// Update updates the task configuration contained in the specified file to match the
	// provided definition.
	//
	// Certain task definitions cannot be updated (see `CanUpdate()` below), in which case an error
	// will be returned.
	Update(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error

	// CanUpdate checks if the task configuration contained in the specified file can be automatically updated.
	//
	// If this method returns `false`, calling `Update()` on this file will return an error.
	//
	// For example, the following JavaScript task definition cannot be automatically updated because it
	// includes programmatic logic:
	//
	//   export default airplane.task({
	//     allowSelfApprovals: process.env.AIRPLANE_ENV_SLUG !== "prod"
	//     // ...
	//   }, /*...*/)
	//
	// If this task definition was updated, the logic would be overwritten with a computed
	// value for `allowSelfApprovals`. Therefore, we don't allow automatic updates of this task definition.
	CanUpdate(ctx context.Context, logger logger.Logger, path string, slug string) (bool, error)
}

type PrepareRunOptions struct {
	// Path is the file path leading to the task's entrypoint.
	//
	// It should be an absolute path.
	Path string
	// WorkingDir is the working directory the local run is being executed in
	WorkingDir string
	// ParamValues specifies the user-provided parameter values to
	// execute this run with.
	ParamValues Values

	// KindOptions specifies any runtime-specific task configuration.
	KindOptions buildtypes.KindOptions

	TaskSlug string
	RunID    string

	// Optional builtin client for runtimes that need it (SQL, Rest, builtin).
	BuiltinsClient *builtins.LocalBuiltinClient
}

// Runtimes is a collection of registered runtimes.
//
// The key is the file extension used for the runtime.
var runtimes = make(map[string]Interface)

// Register registers the given ext with r.
func Register(ext string, r Interface) {
	if _, ok := runtimes[ext]; ok {
		panic(fmt.Sprintf("runtime: %s already registered", ext))
	}
	runtimes[ext] = r
}

// Lookup returns a runtime by kind and path.
// If an extension match is found, use that runtime. Otherwise rely on the task kind.
func Lookup(path string, kind buildtypes.TaskKind) (Interface, error) {
	ext := filepath.Ext(path)
	if runtime, ok := runtimes[ext]; ok {
		return runtime, nil
	}

	// There was no exact match on the extension. Fallback to checking if there
	// is exactly one match on the task kind, which can occur for task kinds that
	// support arbitrary extensions (f.e. shell).
	possible := []Interface{}
	for _, runtime := range runtimes {
		if runtime.Kind() == kind {
			possible = append(possible, runtime)
		}
	}
	if len(possible) > 1 {
		return nil, errors.Errorf("found %d runtimes for task type at path %s, expecting 1", len(possible), path)
	}
	if len(possible) == 0 {
		return nil, errors.New("did not find any runtimes for task type")
	}
	return possible[0], nil
}

// SuggestExts returns a list of extensions for a given TaskKind. May be empty.
func SuggestExts(kind buildtypes.TaskKind) []string {
	exts := []string{}
	for ext, runtime := range runtimes {
		if runtime.Kind() == kind {
			exts = append(exts, ext)
		}
	}
	// Sort, so the return value is deterministic.
	sort.Strings(exts)
	return exts
}

func SuggestKind(ext string) (buildtypes.TaskKind, error) {
	if runtime, ok := runtimes[ext]; ok {
		return runtime.Kind(), nil
	}
	return "", errors.New("No kind to suggest")
}

func RootForNonBuiltRuntime(path string) (string, error) {
	configPath, found := fsx.Find(path, config.FileName)
	if found {
		return configPath, nil
	}
	return filepath.Dir(path), nil
}
