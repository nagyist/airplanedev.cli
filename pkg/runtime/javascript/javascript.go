package javascript

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/airplanedev/lib/pkg/build/node"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	buildversions "github.com/airplanedev/lib/pkg/build/versions"
	"github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	deployutils "github.com/airplanedev/lib/pkg/deploy/utils"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/runtime/updaters"
	"github.com/airplanedev/lib/pkg/utils/airplane_directory"
	"github.com/airplanedev/lib/pkg/utils/cryptox"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

// Init register the runtime.
func init() {
	runtime.Register(".js", Runtime{})
	runtime.Register(".jsx", Runtime{})
}

const (
	depHashFile = "dep-hash"
)

//go:embed updater/index.js
var updaterScript []byte

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

func paramRequired(param definitions.ParameterDefinition) bool {
	return param.Required.Value()
}

// Inline code template.
var inlineCode = template.Must(template.New("ts").Funcs(template.FuncMap{
	"escape":        escapeString,
	"toJSTypeVar":   toJavascriptTypeVar,
	"paramRequired": paramRequired,
}).Parse(`import airplane from "airplane"

export default airplane.task(
	{
		slug: "{{.Slug}}",
		{{- with .Name}}
		name: "{{escape .}}",
		{{- end}}
		{{- with .Description}}
		description: "{{escape .}}",
		{{- end}}
		{{- if .Workflow}}
		// To learn more about the workflow runtime, see the runtime docs:
		// https://docs.airplane.dev/tasks/runtimes
		runtime: "workflow",
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
		{{- if and (ne .Timeout 3600) (gt .Timeout 0) }}
		timeout: {{.Timeout}},
		{{- end}}
		{{- if .Constraints}}
		constraints: {
		{{- range $key, $value := .Constraints}}
			"{{escape $key}}": "{{escape $value}}",
		{{- end}}
		},
		{{- end}}
		{{- if .Resources}}
		resources: {
		{{- range $key, $value := .Resources}}
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
	*definitions.Definition
	AllowSelfApprovals bool
	Timeout            int
	Workflow           bool
}

// GenerateInline implementation.
func (r Runtime) GenerateInline(def *definitions.Definition) ([]byte, fs.FileMode, error) {
	var buf bytes.Buffer
	helper := inlineHelper{
		Definition:         def,
		AllowSelfApprovals: def.AllowSelfApprovals.Value(),
		Timeout:            def.Timeout,
		Workflow:           def.Runtime == buildtypes.TaskRuntimeWorkflow,
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

	return runtime.RootForNonBuiltRuntime(path)
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
	pkg, err := node.ReadPackageJSON(root)
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

func (r Runtime) Version(rootPath string) (buildVersion buildtypes.BuildTypeVersion, err error) {
	pkg, err := node.ReadPackageJSON(rootPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if pkg.Engines.NodeVersion != "" {
		// Look for version in package.json
		nodeConstraint, err := semver.NewConstraint(pkg.Engines.NodeVersion)
		if err != nil {
			return "", errors.Wrapf(err, "parsing node engine %s", pkg.Engines.NodeVersion)
		}

		v, err := buildversions.GetVersions()
		if err != nil {
			return "", err
		}
		supportedVersionsMap := v[string(buildtypes.NameNode)]
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
				return buildtypes.BuildTypeVersion(supportedVersion), nil
			}
		}
	}

	// Look for version in airplane.config
	hasAirplaneConfig := config.HasAirplaneConfig(rootPath)
	if hasAirplaneConfig {
		c, err := config.NewAirplaneConfigFromFile(rootPath)
		if err == nil && c.Javascript.NodeVersion != "" {
			return buildtypes.BuildTypeVersion(c.Javascript.NodeVersion), nil
		}
	}

	return "", nil
}

// Kind implementation.
func (r Runtime) Kind() buildtypes.TaskKind {
	return buildtypes.TaskKindNode
}

func (r Runtime) FormatComment(s string) string {
	lines := []string{}
	for _, line := range strings.Split(s, "\n") {
		lines = append(lines, "// "+line)
	}
	return strings.Join(lines, "\n")
}

func (r Runtime) PrepareRun(
	ctx context.Context,
	logger logger.Logger,
	opts runtime.PrepareRunOptions,
) (rexprs []string, rcloser io.Closer, rerr error) {
	start := time.Now()
	checkNodeVersion(ctx, logger, opts.KindOptions)

	root, err := r.Root(opts.Path)
	if err != nil {
		return nil, nil, err
	}

	airplaneDir, taskDir, _, err := airplane_directory.CreateTaskDir(root, opts.TaskSlug)
	if err != nil {
		return nil, nil, err
	}

	shim := node.UniversalNodeShim
	shimPath := filepath.Join(taskDir, "shim.js")
	if err := os.WriteFile(shimPath, []byte(shim), 0644); err != nil {
		return nil, nil, errors.Wrap(err, "writing shim file")
	}

	// Install the dependencies we need for our shim file:
	rootPackageJSON := filepath.Join(root, "package.json")
	packageJSONs, usesWorkspaces, err := node.GetPackageJSONs(rootPackageJSON)
	if err != nil {
		return nil, nil, errors.Wrap(err, "getting package JSONs")
	}

	pjson, err := node.GenShimPackageJSON(node.GenShimPackageJSONOpts{
		RootDir:      root,
		PackageJSONs: packageJSONs,
		IsWorkflow:   false,
		IsBundle:     true,
	})
	if err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(filepath.Join(airplaneDir, "package.json"), pjson, 0644); err != nil {
		return nil, nil, errors.Wrap(err, "writing shim package.json")
	}

	buildDepsEqual, err := CheckDepHash(airplaneDir)
	if err != nil {
		return nil, nil, err
	}

	// Check if shim dependencies are installed already _and_ if they are up-to-date.
	if _, err := os.Stat(filepath.Join(airplaneDir, "node_modules")); (err == nil && !buildDepsEqual) || errors.Is(err, os.ErrNotExist) {
		shimDepInstallStart := time.Now()
		cmd := exec.CommandContext(ctx, "npm", "install")
		cmd.Dir = airplaneDir
		logger.Debug("Running %s (in %s)", strings.Join(cmd.Args, " "), cmd.Dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			logger.Log(strings.TrimSpace(string(out)))
			return nil, nil, errors.New("failed to install shim deps")
		}
		logger.Debug("Installed shim deps in %s", time.Since(shimDepInstallStart))
	} else if err == nil {
		logger.Debug("Shim deps already installed")
	} else {
		return nil, nil, errors.Wrap(err, "checking for existing shim")
	}

	// [Legacy] We used to build a shim directly into .airplane/dist/, but this logic has since been moved to individual
	// run subdirectories. Remove the old dist/ folder if it exists.
	if err := os.RemoveAll(filepath.Join(airplaneDir, "dist")); err != nil {
		return nil, nil, errors.Wrap(err, "cleaning dist folder")
	}

	// Workaround to get esbuild to not bundle dependencies.
	// See build.ExternalPackages for details.
	externalDeps, err := node.ExternalPackages(packageJSONs, usesWorkspaces)
	if err != nil {
		return nil, nil, err
	}
	externalDepsData, err := json.Marshal(externalDeps)
	if err != nil {
		return nil, nil, errors.Wrap(err, "marshaling external deps")
	}
	logger.Debug("Discovered external dependencies: %v", externalDeps)

	// Save esbuild.js which we will use to run the build.
	esBuildPath := filepath.Join(airplaneDir, "esbuild.js")
	if err := os.WriteFile(esBuildPath, []byte(node.Esbuild), 0644); err != nil {
		return nil, nil, errors.Wrap(err, "writing esbuild file")
	}

	// Check if shim exists; if it does, skip building the shim and just use the existing one.
	builtShimPath := filepath.Join(taskDir, "dist/shim.js")
	if _, err := os.Stat(builtShimPath); (err == nil && !buildDepsEqual) || errors.Is(err, os.ErrNotExist) {
		shimEntrypoints := []string{shimPath}
		shimEntrypointsData, err := json.Marshal(shimEntrypoints)
		if err != nil {
			return nil, nil, errors.Wrap(err, "marshaling shim entrypoints")
		}

		// First build the shim.
		shimBuildStart := time.Now()
		cmd := exec.CommandContext(ctx,
			"node",
			esBuildPath,
			string(shimEntrypointsData),
			"node"+node.GetNodeVersion(opts.KindOptions),
			string(externalDepsData),
			builtShimPath,
		)
		cmd.Dir = airplaneDir
		logger.Debug("Running %s (in %s)", strings.Join(cmd.Args, " "), cmd.Dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			logger.Log(strings.TrimSpace(string(out)))
			return nil, nil, errors.New("failed to build task shim")
		}
		logger.Debug("Built task shim in %s", time.Since(shimBuildStart))
	} else if err == nil {
		logger.Debug("Using existing shim")
	} else {
		return nil, nil, errors.Wrap(err, "checking for existing shim")
	}

	// Then build the entrypoint. We always do this since this is unique per run.
	entrypoint, err := filepath.Rel(root, opts.Path)
	if err != nil {
		return nil, nil, errors.Wrap(err, "entrypoint is not within the task root")
	}
	entrypoints := []string{filepath.Join(root, entrypoint)}
	entrypointsData, err := json.Marshal(entrypoints)
	if err != nil {
		return nil, nil, errors.Wrap(err, "marshaling entrypoints")
	}

	runDir, closer, err := airplane_directory.CreateRunDir(taskDir, opts.RunID)
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

	entrypointBuildStart := time.Now()
	cmd := exec.CommandContext(ctx,
		"node",
		esBuildPath,
		string(entrypointsData),
		"node"+node.GetNodeVersion(opts.KindOptions),
		string(externalDepsData),
		"",
		filepath.Join(runDir, "dist"),
		root,
	)
	cmd.Dir = airplaneDir
	logger.Debug("Running %s (in %s)", strings.Join(cmd.Args, " "), airplaneDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log(strings.TrimSpace(string(out)))
		return nil, nil, errors.New("failed to build task")
	}

	logger.Debug("Built entrypoint in %s", time.Since(entrypointBuildStart).String())

	pv, err := json.Marshal(opts.ParamValues)
	if err != nil {
		return nil, nil, errors.Wrap(err, "serializing param values")
	}

	entrypointFunc, _ := opts.KindOptions["entrypointFunc"].(string)
	entrypointExt := filepath.Ext(entrypoint)
	entrypointJS := strings.TrimSuffix(entrypoint, entrypointExt) + ".js"

	logger.Debug("Prepared run for execution in %s", time.Since(start))

	return []string{"node", builtShimPath, filepath.Join(runDir, "dist", entrypointJS), entrypointFunc, string(pv)}, closer, nil
}

// SupportsLocalExecution implementation.
func (r Runtime) SupportsLocalExecution() bool {
	return true
}

var airplaneErrorRegex = regexp.MustCompile("__airplane_error (.*)\n")
var airplaneOutputRegex = regexp.MustCompile("__airplane_output (.*)\n")

func (r Runtime) Update(ctx context.Context, logger logger.Logger, path string, slug string, def definitions.Definition) error {
	if deployutils.IsNodeInlineAirplaneEntity(path) {
		if _, err := os.Stat(path); err != nil {
			return errors.Wrap(err, "opening file")
		}

		tempFile, err := os.CreateTemp("", "airplane.transformer-js-*")
		if err != nil {
			return errors.Wrap(err, "creating temporary file")
		}
		defer os.Remove(tempFile.Name())
		_, err = tempFile.Write(updaterScript)
		if err != nil {
			return errors.Wrap(err, "writing script")
		}

		defJSON, err := def.Marshal(definitions.DefFormatJSON)
		if err != nil {
			return errors.Wrap(err, "marshalling definition as JSON")
		}

		_, err = runNodeCommand(ctx, logger, updaterScript, "update", path, slug, string(defJSON))
		if err != nil {
			return errors.WithMessagef(err, "updating task at %q (re-run with --debug for more context)", path)
		}

		return nil
	}

	return updaters.UpdateYAML(ctx, logger, path, slug, def)
}

func (r Runtime) CanUpdate(ctx context.Context, logger logger.Logger, path string, slug string) (bool, error) {
	if deployutils.IsNodeInlineAirplaneEntity(path) {
		if _, err := os.Stat(path); err != nil {
			return false, errors.Wrap(err, "opening file")
		}

		out, err := runNodeCommand(ctx, logger, updaterScript, "can_update", path, slug)
		if err != nil {
			return false, errors.WithMessagef(err, "checking if task can be updated at %q (re-run with --debug for more context)", path)
		}

		var canEdit bool
		if err := json.Unmarshal([]byte(out), &canEdit); err != nil {
			return false, errors.Wrap(err, "checking if task can be updated")
		}

		return canEdit, nil
	}

	return updaters.CanUpdateYAML(path)
}

func runNodeCommand(ctx context.Context, logger logger.Logger, script []byte, args ...string) (string, error) {
	tempFile, err := os.CreateTemp("", "airplane.runtime.javascript-*")
	if err != nil {
		return "", errors.Wrap(err, "creating temporary file")
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.Write(script)
	if err != nil {
		return "", errors.Wrap(err, "writing script")
	}

	allArgs := append([]string{tempFile.Name()}, args...)
	cmd := exec.Command("node", allArgs...)
	logger.Debug("Running %s", strings.Join(cmd.Args, " "))

	out, err := cmd.Output()
	if len(out) == 0 {
		out = []byte("(none)")
	}
	logger.Debug("Output:\n%s", out)

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			matches := airplaneErrorRegex.FindStringSubmatch(string(ee.Stderr))
			if len(matches) >= 2 {
				errMsg := matches[1]
				return "", errors.New(errMsg)
			}
		}
		return "", errors.Wrap(err, "running node command")
	}

	matches := airplaneOutputRegex.FindStringSubmatch(string(out))
	if len(matches) >= 2 {
		msg := matches[1]
		return msg, nil
	}

	return "", nil
}

// checkNodeVersion compares the major version of the currently installed
// node binary with that of the configured task and logs a warning if they
// do not match.
func checkNodeVersion(ctx context.Context, logger logger.Logger, opts buildtypes.KindOptions) {
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

// CheckDepHash checks that the hash inside .airplane/dep-hash matches the hash of the build dependencies and build
// script.
func CheckDepHash(airplaneDir string) (bool, error) {
	var prevHash string
	contents, err := os.ReadFile(filepath.Join(airplaneDir, depHashFile))
	if err != nil {
		if os.IsNotExist(err) {
			prevHash = ""
		} else {
			return false, errors.Wrap(err, "reading dependency hash file")
		}
	} else {
		prevHash = string(contents)
	}

	currHash, err := cryptox.ComputeHashFromFiles(
		filepath.Join(airplaneDir, "package.json"),
		filepath.Join(airplaneDir, "esbuild.js"),
	)
	if err != nil {
		return false, err
	}

	if prevHash == currHash {
		return true, nil
	} else {
		if err := os.WriteFile(filepath.Join(airplaneDir, depHashFile), []byte(currHash), 0644); err != nil {
			return false, errors.Wrap(err, "writing dependency hash file")
		}
		return false, nil
	}
}
