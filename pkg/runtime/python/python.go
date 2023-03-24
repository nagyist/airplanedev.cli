package python

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/airplanedev/lib/pkg/build/python"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/deploy/utils"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/runtime/updaters"
	"github.com/airplanedev/lib/pkg/utils/airplane_directory"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

// Init register the runtime.
func init() {
	runtime.Register(".py", Runtime{})
}

// Code template.
var code = template.Must(template.New("py").Parse(`{{with .Comment -}}
{{.}}

{{end -}}
# This is your task's entrypoint. When your task is executed, this
# function will be called.
def main(params):
    data = [
        {"id": 1, "name": "Gabriel Davis", "role": "Dentist"},
        {"id": 2, "name": "Carolyn Garcia", "role": "Sales"},
        {"id": 3, "name": "Frances Hernandez", "role": "Astronaut"},
        {"id": 4, "name": "Melissa Rodriguez", "role": "Engineer"},
        {"id": 5, "name": "Jacob Hall", "role": "Engineer"},
        {"id": 6, "name": "Andrea Lopez", "role": "Astronaut"},
    ]

    # Sort the data in ascending order by name.
    data = sorted(data, key=lambda u: u["name"])

    # You can return data to show output to users.
    # Output documentation: https://docs.airplane.dev/tasks/output
    return data
`))

// Data represents the data template.
type data struct {
	Comment string
}

// Runtime implementation.
type Runtime struct{}

// PrepareRun implementation.
func (r Runtime) PrepareRun(ctx context.Context, logger logger.Logger, opts runtime.PrepareRunOptions) (rexprs []string, rcloser io.Closer, rerr error) {
	if err := checkPythonInstalled(ctx, logger); err != nil {
		return nil, nil, err
	}

	root, err := r.Root(opts.Path)
	if err != nil {
		return nil, nil, err
	}

	_, taskDir, closer, err := airplane_directory.CreateTaskDir(root, opts.TaskSlug)
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		// If we encountered an error before returning, then we're responsible
		// for performing our own cleanup.
		if rerr != nil {
			closer.Close()
		}
	}()

	entrypoint, err := filepath.Rel(root, opts.Path)
	if err != nil {
		return nil, nil, errors.Wrap(err, "entrypoint is not within the task root")
	}
	entrypointFunc, _ := opts.KindOptions["entrypointFunc"].(string)
	shim, err := python.PythonShim(python.PythonShimParams{
		TaskRoot:       root,
		Entrypoint:     entrypoint,
		EntrypointFunc: entrypointFunc,
	})
	if err != nil {
		return nil, nil, err
	}

	if err := os.WriteFile(filepath.Join(taskDir, "shim.py"), []byte(shim), 0644); err != nil {
		return nil, nil, errors.Wrap(err, "writing shim file")
	}

	pv, err := json.Marshal(opts.ParamValues)
	if err != nil {
		return nil, nil, errors.Wrap(err, "serializing param values")
	}

	bin := pythonBin(logger)
	if bin == "" {
		return nil, nil, errors.New("could not find python")
	}
	// -u forces the stdout stream to be unbuffered, or else Python may buffer logs until the run completes.
	return []string{pythonBin(logger), "-u", filepath.Join(taskDir, "shim.py"), string(pv)}, closer, nil
}

// pythonBin returns the first of python3 or python found on PATH, if any.
// We expect most systems to have python3 if Python 3 is installed, as per PEP 0394:
// https://www.python.org/dev/peps/pep-0394/#recommendation
// However, Python on Windows (whether through Python or Anaconda) does not seem to install python3.exe.
func pythonBin(logger logger.Logger) string {
	for _, bin := range []string{"python3", "python"} {
		logger.Debug("Looking for binary %s", bin)
		path, err := exec.LookPath(bin)
		if err == nil {
			logger.Debug("Found binary %s at %s", bin, path)
			return bin
		}
		logger.Debug("Could not find binary %s: %s", bin, err)
	}
	return ""
}

// Checks that Python 3 is installed, since we rely on 3 and don't support 2.
func checkPythonInstalled(ctx context.Context, logger logger.Logger) error {
	bin := pythonBin(logger)
	if bin == "" {
		return errors.New(heredoc.Doc(`
            Could not find the python3 or python commands on your PATH.
            Ensure that Python 3 is installed and available in your shell environment.
        `))
	}
	cmd := exec.CommandContext(ctx, bin, "--version")
	logger.Debug("Running %s", strings.Join(cmd.Args, " "))
	out, err := cmd.Output()
	if err != nil {
		return errors.New(fmt.Sprintf(heredoc.Doc(`
            Got an error while running %s:
            %s
        `), strings.Join(cmd.Args, " "), err.Error()))
	}
	version := string(out)
	if !strings.HasPrefix(version, "Python 3.") {
		return errors.New(fmt.Sprintf(heredoc.Doc(`
            Could not find Python 3 on your PATH. Found %s but running --version returned: %s
        `), bin, version))
	}
	return nil
}

// Generate implementation.
func (r Runtime) Generate(t *runtime.Task) ([]byte, fs.FileMode, error) {
	d := data{}
	if t != nil {
		d.Comment = runtime.Comment(r, t.URL)
	}

	var buf bytes.Buffer
	if err := code.Execute(&buf, d); err != nil {
		return nil, 0, fmt.Errorf("python: template execute - %w", err)
	}

	return buf.Bytes(), 0644, nil
}

func toPythonType(value string) (string, error) {
	switch value {
	case "shorttext":
		return "str", nil
	case "longtext":
		return "airplane.LongText", nil
	case "sql":
		return "airplane.SQL", nil
	case "boolean":
		return "bool", nil
	case "integer":
		return "int", nil
	case "float":
		return "float", nil
	case "file":
		return "str", nil
	case "airplane.File":
		return "str", nil
	case "date":
		return "datetime.date", nil
	case "datetime":
		return "datetime.datetime", nil
	case "configvar":
		return "airplane.ConfigVar", nil
	case "upload":
		return "airplane.File", nil
	}
	return "", errors.Errorf("unsupported type %s", value)
}

func needsDatetimeImport(params []definitions.ParameterDefinition) bool {
	for _, param := range params {
		if param.Type == "date" || param.Type == "datetime" {
			return true
		}
	}
	return false
}

func needsOptionalImport(params []definitions.ParameterDefinition) bool {
	for _, param := range params {
		if !paramIsRequired(param) {
			return true
		}
	}
	return false
}

func toPythonTypeVar(paramType string, paramValue interface{}) (string, error) {
	switch v := paramValue.(type) {
	case nil:
		return "None", nil
	case bool:
		if v {
			return "True", nil
		}
		return "False", nil
	case float64:
		return fmt.Sprintf("%v", v), nil
	case string:
		if paramType == "date" {
			date, err := time.Parse("2006-01-02", v)
			if err != nil {
				return "", errors.Wrap(err, "date")
			}
			return fmt.Sprintf(
				"datetime.date(%d, %d, %d)",
				date.Year(), date.Month(), date.Day(),
			), nil
		} else if paramType == "datetime" {
			dt, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return "", errors.Wrap(err, "unable to parse datetime")
			}
			return fmt.Sprintf(
				"datetime.datetime(%d, %d, %d, %d, %d, %d)",
				dt.Year(), dt.Month(), dt.Day(), dt.Hour(), dt.Minute(), dt.Second(),
			), nil
		} else if paramType == "configVar" {
			return fmt.Sprintf(`airplane.ConfigVar(name=%s, value="")`, strconv.Quote(v)), nil
		}
		return strconv.Quote(v), nil
	default:
		return "", errors.Errorf("unable to handle variable %v of type %T", v, v)
	}
}

func paramIsRequired(param definitions.ParameterDefinition) bool {
	return param.Required.Value()
}

// Inline code template.
var inlineCode = template.Must(template.New("py").Funcs(template.FuncMap{
	"toPythonType":    toPythonType,
	"toPythonTypeVar": toPythonTypeVar,
	"paramIsRequired": paramIsRequired,
	"quote":           strconv.Quote,
}).Parse(`{{- if .NeedsDatetimeImport}}import datetime
{{end}}
{{- if .NeedsOptionalImport }}from typing import Optional
{{end}}
{{- if or .NeedsDatetimeImport .NeedsOptionalImport}}
import airplane
{{- else }}import airplane{{- end }}
{{if .Parameters }}from typing_extensions import Annotated
{{end}}

@airplane.{{ .SDKMethod }}(
    slug={{quote .Slug}},
    name={{quote .Name}},
    {{- if .Description}}
    description={{quote .Description}},
    {{- end}}
    {{- if .RequireRequests}}
    require_requests=True,
    {{- end}}
    {{- if not .AllowSelfApprovals}}
    allow_self_approvals=False,
    {{- end}}
    {{- if and (ne .Timeout 3600) (gt .Timeout 0) }}
    timeout={{.Timeout}},
    {{- end}}
    {{- if .Constraints}}
    constraints={
    {{- range $key, $value := .Constraints}}
        {{quote $key}}: {{quote $value}},
    {{- end}}
    },
    {{- end}}
    {{- if .Resources}}
    resources=[
    {{- range $key, $value := .Resources}}
        airplane.Resource(
            alias={{quote $key}},
            slug={{quote $value}},
        ),
    {{- end}}
    ],
    {{- end}}
    {{- if .Schedules }}
    schedules=[
    {{- range $key, $value := .Schedules}}
        airplane.Schedule(
            slug={{quote $key}},
            cron={{quote $value.CronExpr}},
            {{- if $value.Name}}
            name={{quote $value.Name}},
            {{- end}}
            {{- if $value.Description}}
            description={{quote $value.Description}},
            {{- end}}
            {{- if $value.ParamValues}}
            param_values={
                {{- range $pSlug, $pValue := $value.ParamValues}}
                {{quote $pSlug}}: {{toPythonTypeVar (index $.ParamSlugToType $pSlug) $pValue}},
                {{- end}}
            },
            {{- end}}
        ),
    {{- end}}
    ],
    {{- end}}
    {{- if .Python.EnvVars }}
    env_vars=[
    {{- range $key, $value := .Python.EnvVars}}
    {{- if $value.Value}}
        airplane.EnvVar(
            name={{quote $key}},
            value={{quote $value.Value}},
        ),
    {{- else if $value.Config}}
        airplane.EnvVar(
            name={{quote $key}},
            config_var_name={{quote $value.Config}},
        ),
    {{- end}}
    {{- end}}
    ],
    {{- end}}
)
{{- if .Parameters}}
def {{.Slug}}(
    {{- range $key, $value := .Parameters}}
    {{$value.Slug}}: Annotated[
        {{- if not (paramIsRequired $value)}}
        Optional[{{toPythonType $value.Type}}],
        {{- else }}
        {{toPythonType $value.Type}},
        {{- end}}
        airplane.ParamConfig(
            slug={{quote $value.Slug}},
            {{- if $value.Name}}
            name={{quote $value.Name}},
            {{- end}}
            {{- if $value.Description}}
            description={{quote $value.Description}},
            {{- end}}
            {{- if $value.Options}}
            options=[
                {{- range $oKey, $oValue := $value.Options}}
                {{- if $oValue.Label}}
                airplane.LabeledOption(
                    label={{quote $oValue.Label}},
                    value={{toPythonTypeVar $value.Type $oValue.Value}},
                ),
                {{- else}}
                {{toPythonTypeVar $value.Type $oValue.Value}}},
                {{- end}}
                {{- end}}
            ],
            {{- end}}
            {{- if $value.Regex}}
            regex={{quote $value.Regex}},
            {{- end}}
        ),
    ]{{if ne $value.Default nil}} = {{toPythonTypeVar $value.Type $value.Default}}{{end}},
    {{- end}}
):
{{- else}}
def {{.Slug}}():
{{- end}}
    data = [
        {"id": 1, "name": "Gabriel Davis", "role": "Dentist"},
        {"id": 2, "name": "Carolyn Garcia", "role": "Sales"},
        {"id": 3, "name": "Frances Hernandez", "role": "Astronaut"},
        {"id": 4, "name": "Melissa Rodriguez", "role": "Engineer"},
        {"id": 5, "name": "Jacob Hall", "role": "Engineer"},
        {"id": 6, "name": "Andrea Lopez", "role": "Astronaut"},
    ]

    # Sort the data in ascending order by name.
    data = sorted(data, key=lambda u: u["name"])

    # You can return data to show output to users.
    # Output documentation: https://docs.airplane.dev/tasks/output
    return data
`))

type inlineHelper struct {
	*definitions.Definition
	AllowSelfApprovals  bool
	Timeout             int
	SDKMethod           string
	NeedsOptionalImport bool
	NeedsDatetimeImport bool
	ParamSlugToType     map[string]string
}

// GenerateInline implementation.
func (r Runtime) GenerateInline(def *definitions.Definition) ([]byte, fs.FileMode, error) {
	var buf bytes.Buffer
	method := "task"
	if def.Runtime == buildtypes.TaskRuntimeWorkflow {
		method = "workflow"
	}
	paramSlugToType := map[string]string{}
	for _, param := range def.Parameters {
		paramSlugToType[param.Slug] = param.Type
	}

	helper := inlineHelper{
		Definition:          def,
		AllowSelfApprovals:  def.AllowSelfApprovals.Value(),
		Timeout:             def.Timeout,
		SDKMethod:           method,
		NeedsOptionalImport: needsOptionalImport(def.Parameters),
		NeedsDatetimeImport: needsDatetimeImport(def.Parameters),
		ParamSlugToType:     paramSlugToType,
	}
	if err := inlineCode.Execute(&buf, helper); err != nil {
		return nil, 0, fmt.Errorf("python: template execute - %w", err)
	}

	return buf.Bytes(), 0644, nil
}

// Workdir implementation.
func (r Runtime) Workdir(path string) (string, error) {
	return r.Root(path)
}

// Root implementation.
func (r Runtime) Root(path string) (string, error) {
	root, ok := fsx.Find(path, "requirements.txt")
	if ok {
		return root, nil

	}
	return runtime.RootForNonBuiltRuntime(path)
}

func (r Runtime) Version(rootPath string) (buildVersion buildtypes.BuildTypeVersion, err error) {
	// Look for version in airplane.config
	hasAirplaneConfig := config.HasAirplaneConfig(rootPath)
	if hasAirplaneConfig {
		c, err := config.NewAirplaneConfigFromFile(rootPath)
		if err == nil && c.Python.Version != "" {
			return buildtypes.BuildTypeVersion(c.Python.Version), nil
		}
	}

	return "", nil
}

// Kind implementation.
func (r Runtime) Kind() buildtypes.TaskKind {
	return buildtypes.TaskKindPython
}

// FormatComment implementation.
func (r Runtime) FormatComment(s string) string {
	var lines []string

	for _, line := range strings.Split(s, "\n") {
		lines = append(lines, "# "+line)
	}

	return strings.Join(lines, "\n")
}

// SupportsLocalExecution implementation.
func (r Runtime) SupportsLocalExecution() bool {
	return true
}

func (r Runtime) Update(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error {
	if deployutils.IsPythonInlineAirplaneEntity(path) {
		// TODO(colin, 04012023): support updating inline python
		return errors.New("Support for updating .py files is coming soon.")
	}

	return updaters.UpdateYAML(ctx, logger, path, slug, def)
}

func (r Runtime) CanUpdate(ctx context.Context, logger logger.Logger, path string, slug string) (bool, error) {
	if deployutils.IsPythonInlineAirplaneEntity(path) {
		// TODO(colin, 04012023): support updating inline python
		return false, errors.New("Support for updating .py files is coming soon.")
	}

	return updaters.CanUpdateYAML(path)
}
