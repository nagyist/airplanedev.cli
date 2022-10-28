package javascript

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
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/lib/pkg/utils/logger"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

// Init register the runtime.
func init() {
	runtime.Register(".js", Runtime{})
	runtime.Register(".jsx", Runtime{})
}

// Code template.
var code = template.Must(template.New("js").Parse(`{{with .Comment -}}
{{.}}

{{end -}}
// This is your task's entrypoint. When your task is executed, this
// function will be called.
export default async function(params) {
	const data = [
		{ id: 1, name: "Gabriel Davis", role: "Dentist" },
		{ id: 2, name: "Carolyn Garcia", role: "Sales" },
		{ id: 3, name: "Frances Hernandez", role: "Astronaut" },
		{ id: 4, name: "Melissa Rodriguez", role: "Engineer" },
		{ id: 5, name: "Jacob Hall", role: "Engineer" },
		{ id: 6, name: "Andrea Lopez", role: "Astronaut" },
	];

	// Sort the data in ascending order by name.
	data.sort((u1, u2) => {
		return u1.name.localeCompare(u2.name);
	});

	// You can return data to show output to users.
	// Output documentation: https://docs.airplane.dev/tasks/output
	return data;
}
`))

// Inline code template function helpers.
func escapeString(s string) (string, error) {
	val, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(val[1 : len(val)-1]), nil
}

func toJavascriptTypeVar(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		val, err := escapeString(v)
		return fmt.Sprintf("\"%s\"", val), err
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func paramRequired(param definitions.ParameterDefinition_0_3) bool {
	return param.Required.Value()
}

// Inline code template.
var inlineCode = template.Must(template.New("ts").Funcs(template.FuncMap{
	"escape":        escapeString,
	"toJSTypeVar":   toJavascriptTypeVar,
	"paramRequired": paramRequired,
}).Parse(`import airplane from "airplane"

export default airplane.{{ .SDKMethod }}(
	{
		slug: "{{.Slug}}",
		{{- with .Name}}
		name: "{{escape .}}",
		{{- end}}
		{{- with .Description}}
		description: "{{escape .}}",
		{{- end}}
		{{- if .Parameters}}
		parameters: {
		{{- range $key, $value := .Parameters}}
			{{$value.Slug}}: {
				type: "{{$value.Type}}",
				{{- if $value.Name}}
				name: "{{escape $value.Name}}",
				{{- end}}
				{{- if $value.Description}}
				description: "{{escape $value.Description}}",
				{{- end}}
				{{- if not (paramRequired $value)}}
				required: false,
				{{- end}}
				{{- if ne $value.Default nil}}
				default: {{toJSTypeVar $value.Default}},
				{{- end}}
				{{- if $value.Options}}
				options: [
					{{- range $oKey, $oValue := $value.Options}}
					{{- if $oValue.Label}}
					{label: "{{escape $oValue.Label}}", value: {{toJSTypeVar $oValue.Value}}},
					{{- else}}
					{{toJSTypeVar $oValue.Value}},
					{{- end}}
					{{- end}}
				]
				{{- end}}
				{{- if $value.Regex}}
				regex: "{{escape $value.Regex}}",
				{{- end}}
			},
		{{- end}}
		},
		{{- end}}
		{{- if .RequireRequests}}
		requireRequests: true,
		{{- end}}
		{{- if not .AllowSelfApprovals}}
		allowSelfApprovals: false,
		{{- end}}
		{{- if ne .Timeout 3600}}
		timeout: {{.Timeout}},
		{{- end}}
		{{- if .Constraints}}
		constraints: {
		{{- range $key, $value := .Constraints}}
			"{{escape $key}}": "{{escape $value}}",
		{{- end}}
		},
		{{- end}}
		{{- if .Resources.Attachments}}
		resources: {
		{{- range $key, $value := .Resources.Attachments}}
			"{{escape $key}}": "{{$value}}",
		{{- end}}
		},
		{{- end}}
		{{- if .Schedules }}
		schedules: {
		{{- range $key, $value := .Schedules}}
			{{$key}}: {
				cron: "{{$value.CronExpr}}",
				{{- if $value.Name}}
				name: "{{escape $value.Name}}",
				{{- end}}
				{{- if $value.Description}}
				description: "{{escape $value.Description}}",
				{{- end}}
				{{- if $value.ParamValues}}
				paramValues: {
					{{range $pSlug, $pValue := $value.ParamValues}}
					{{- $pSlug}}: {{toJSTypeVar $pValue}},
					{{- end}}
				}
				{{- end}}
			}
		{{- end}}
		},
		{{- end}}
		{{- if and (not (eq .SDKMethod "workflow")) .Node.NodeVersion }}
		nodeVersion: "{{.Node.NodeVersion}}",
		{{- end}}
		{{- if .Node.EnvVars }}
		envVars: {
		{{- range $key, $value := .Node.EnvVars}}
		{{- if $value.Value}}
			"{{escape $key}}": "{{escape $value.Value}}",
		{{- else if $value.Config}}
			"{{escape $key}}": {
				config: "{{escape $value.Config}}"
			},
		{{- end}}
		{{- end}}
		},
		{{- end}}
	},
	// This is your task's entrypoint. When your task is executed, this
	// function will be called.
	{{- if .Parameters}}
	async (params) => {
	{{- else}}
	async () => {
	{{- end}}
		const data = [
			{ id: 1, name: "Gabriel Davis", role: "Dentist" },
			{ id: 2, name: "Carolyn Garcia", role: "Sales" },
			{ id: 3, name: "Frances Hernandez", role: "Astronaut" },
			{ id: 4, name: "Melissa Rodriguez", role: "Engineer" },
			{ id: 5, name: "Jacob Hall", role: "Engineer" },
			{ id: 6, name: "Andrea Lopez", role: "Astronaut" },
		];

		// Sort the data in ascending order by name.
		data.sort((u1, u2) => {
			return u1.name.localeCompare(u2.name);
		});

		// You can return data to show output to users.
		// Output documentation: https://docs.airplane.dev/tasks/output
		return data;
	}
)
`))

// Data represents the data template.
type data struct {
	Comment string
}

// Runtime implementation.
type Runtime struct{}

// Generate implementation.
func (r Runtime) Generate(t *runtime.Task) ([]byte, fs.FileMode, error) {
	d := data{}
	if t != nil {
		d.Comment = runtime.Comment(r, t.URL)
	}

	var buf bytes.Buffer
	if err := code.Execute(&buf, d); err != nil {
		return nil, 0, fmt.Errorf("javascript: template execute - %w", err)
	}

	return buf.Bytes(), 0644, nil
}

type inlineHelper struct {
	*definitions.Definition_0_3
	AllowSelfApprovals bool
	Timeout            int
	SDKMethod          string
}

// GenerateInline implementation.
func (r Runtime) GenerateInline(def *definitions.Definition_0_3) ([]byte, fs.FileMode, error) {
	var buf bytes.Buffer
	method := "task"
	if def.Runtime == build.TaskRuntimeWorkflow {
		method = "workflow"
	}
	helper := inlineHelper{
		Definition_0_3:     def,
		AllowSelfApprovals: def.AllowSelfApprovals.Value(),
		Timeout:            def.Timeout.Value(),
		SDKMethod:          method,
	}
	if err := inlineCode.Execute(&buf, helper); err != nil {
		return nil, 0, fmt.Errorf("javascript: template execute - %w", err)
	}

	return buf.Bytes(), 0644, nil
}

// Workdir picks the working directory for commands to be executed from.
//
// For JS, that is the nearest parent directory containing a `package.json`.
func (r Runtime) Workdir(path string) (string, error) {
	if p, ok := fsx.Find(path, "package.json"); ok {
		return p, nil
	}

	// Otherwise default to immediate directory of path
	return filepath.Dir(path), nil
}

// Root picks which directory to use as the root of a task's code.
// All code in that directory will be available at runtime.
//
// For JS, this is usually just the workdir. However, this can be overridden
// with the `airplane.root` property in the `package.json`.
func (r Runtime) Root(path string) (string, error) {
	// By default, the root is the workdir.
	root, err := r.Workdir(path)
	if err != nil {
		return "", err
	}

	// Unless the root is overridden with an `airplane.root` field
	// in a `package.json`.
	pkg, err := build.ReadPackageJSON(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// No package.json, use workdir as root.
			return root, nil
		}
		return "", err
	}

	if pkgjsonRoot := pkg.Settings.Root; pkgjsonRoot != "" {
		return filepath.Join(root, pkgjsonRoot), nil
	}

	return root, nil
}

func (r Runtime) Version(rootPath string) (buildVersion build.BuildTypeVersion, err error) {
	pkg, err := build.ReadPackageJSON(rootPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if pkg.Engines.NodeVersion != "" {
		// Look for version in package.json
		nodeConstraint, err := semver.NewConstraint(pkg.Engines.NodeVersion)
		if err != nil {
			return "", errors.Wrapf(err, "parsing node engine %s", pkg.Engines.NodeVersion)
		}

		v, err := build.GetVersions()
		if err != nil {
			return "", err
		}
		supportedVersionsMap := v[string(build.NameNode)]
		var supportedVersions []string
		for supportedVersion := range supportedVersionsMap {
			supportedVersions = append(supportedVersions, supportedVersion)
		}
		slices.SortFunc(supportedVersions, func(a, b string) bool {
			return b < a
		})

		for _, supportedVersion := range supportedVersions {
			sv, err := semver.NewVersion(supportedVersion)
			if err != nil {
				return "", err
			}
			if nodeConstraint.Check(sv) {
				return build.BuildTypeVersion(supportedVersion), nil
			}
		}
	}

	// Look for version in airplane.config
	configPath, found := fsx.Find(rootPath, config.FileName)
	if found {
		c, err := config.NewAirplaneConfigFromFile(filepath.Join(configPath, config.FileName))
		if err == nil && c.NodeVersion != "" {
			return c.NodeVersion, nil
		}
	}

	return "", nil
}

// Kind implementation.
func (r Runtime) Kind() build.TaskKind {
	return build.TaskKindNode
}

func (r Runtime) FormatComment(s string) string {
	lines := []string{}
	for _, line := range strings.Split(s, "\n") {
		lines = append(lines, "// "+line)
	}
	return strings.Join(lines, "\n")
}

func (r Runtime) PrepareRun(ctx context.Context, logger logger.Logger, opts runtime.PrepareRunOptions) (rexprs []string, rcloser io.Closer, rerr error) {
	checkNodeVersion(ctx, logger, opts.KindOptions)

	root, err := r.Root(opts.Path)
	if err != nil {
		return nil, nil, err
	}

	airplaneDir, taskDir, closer, err := runtime.CreateTaskDir(root, opts.TaskSlug)
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
	shim, err := build.TemplatedNodeShim(build.NodeShimParams{
		Entrypoint:     filepath.Join("..", entrypoint),
		EntrypointFunc: entrypointFunc,
	})
	if err != nil {
		return nil, nil, err
	}

	if err := os.WriteFile(filepath.Join(taskDir, "shim.js"), []byte(shim), 0644); err != nil {
		return nil, nil, errors.Wrap(err, "writing shim file")
	}

	// Install the dependencies we need for our shim file:
	rootPackageJSON := filepath.Join(root, "package.json")
	packageJSONs, usesWorkspaces, err := build.GetPackageJSONs(rootPackageJSON)
	if err != nil {
		return nil, nil, errors.Wrap(err, "getting package JSONs")
	}

	pjson, err := build.GenShimPackageJSON(root, packageJSONs, false)
	if err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(filepath.Join(airplaneDir, "package.json"), pjson, 0644); err != nil {
		return nil, nil, errors.Wrap(err, "writing shim package.json")
	}
	cmd := exec.CommandContext(ctx, "npm", "install")
	cmd.Dir = filepath.Join(root, ".airplane")
	logger.Debug("Running %s (in %s)", strings.Join(cmd.Args, " "), root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log(strings.TrimSpace(string(out)))
		return nil, nil, errors.New("failed to install shim deps")
	}

	if err := os.RemoveAll(filepath.Join(airplaneDir, "dist")); err != nil {
		return nil, nil, errors.Wrap(err, "cleaning dist folder")
	}

	// Workaround to get esbuild to not bundle dependencies.
	// See build.ExternalPackages for details.
	externalDeps, err := build.ExternalPackages(packageJSONs, usesWorkspaces)
	if err != nil {
		return nil, nil, err
	}
	logger.Debug("Discovered external dependencies: %v", externalDeps)

	start := time.Now()
	res := esbuild.Build(esbuild.BuildOptions{
		Bundle: true,

		EntryPoints: []string{filepath.Join(taskDir, "shim.js")},
		Outfile:     filepath.Join(taskDir, "dist/shim.js"),
		Write:       true,

		External: externalDeps,
		Platform: esbuild.PlatformNode,
		Engines: []esbuild.Engine{
			// esbuild is relatively generous in the node versions it supports:
			// https://esbuild.github.io/api/#target
			{Name: esbuild.EngineNode, Version: build.GetNodeVersion(opts.KindOptions)},
		},
	})
	for _, w := range res.Warnings {
		logger.Debug("esbuild(warn): %v", w)
	}
	for _, e := range res.Errors {
		logger.Warning("esbuild(error): %v", e)
	}
	logger.Debug("Compiled JS in %s", time.Since(start).String())

	pv, err := json.Marshal(opts.ParamValues)
	if err != nil {
		return nil, nil, errors.Wrap(err, "serializing param values")
	}

	if len(res.OutputFiles) == 0 {
		return nil, nil, errors.New("esbuild failed: see logs")
	}

	return []string{"node", res.OutputFiles[0].Path, string(pv)}, closer, nil
}

// SupportsLocalExecution implementation.
func (r Runtime) SupportsLocalExecution() bool {
	return true
}

// checkNodeVersion compares the major version of the currently installed
// node binary with that of the configured task and logs a warning if they
// do not match.
func checkNodeVersion(ctx context.Context, logger logger.Logger, opts build.KindOptions) {
	nodeVersion, ok := opts["nodeVersion"].(string)
	if !ok {
		return
	}

	v, err := semver.NewVersion(nodeVersion)
	if err != nil {
		logger.Debug("Unable to parse node version (%s): ignoring", nodeVersion)
		return
	}

	cmd := exec.CommandContext(ctx, "node", "--version")
	logger.Debug("Running %s", strings.Join(cmd.Args, " "))
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Debug("failed to check node version: is node installed?")
		return
	}

	logger.Debug("node version: %s", strings.TrimSpace(string(out)))
	if !strings.HasPrefix(string(out), fmt.Sprintf("v%d", v.Major())) {
		logger.Warning("Your local version of Node (%s) does not match the version your task is configured to run against (v%s).", strings.TrimSpace(string(out)), v)
	}

	cmd = exec.CommandContext(ctx, "npx", "--version")
	logger.Debug("Running %s", strings.Join(cmd.Args, " "))
	out, err = cmd.CombinedOutput()
	if err != nil {
		logger.Debug("failed to check npx version: are you running a recent enough version of node?")
		return
	}

	logger.Debug("npx version: %s", strings.TrimSpace(string(out)))
}
