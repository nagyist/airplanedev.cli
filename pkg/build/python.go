package build

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/deploy/discover/parser"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

const (
	DefaultPythonVersion = BuildTypeVersionPython310
)

// Python creates a dockerfile for Python.
func python(
	root string,
	opts KindOptions,
	buildArgs []string,
) (string, error) {
	if opts["shim"] != "true" {
		return pythonLegacy(root, opts)
	}

	// Assert that the entrypoint file exists:
	entrypoint, _ := opts["entrypoint"].(string)
	if err := fsx.AssertExistsAll(filepath.Join(root, entrypoint)); err != nil {
		return "", err
	}

	requirementsPath := filepath.Join(root, "requirements.txt")
	hasRequirements := fsx.Exists(requirementsPath)
	var embeddedRequirements []string
	var err error
	if hasRequirements {
		embeddedRequirements, err = collectEmbeddedRequirements(root, requirementsPath)
		if err != nil {
			return "", err
		}
	}

	var airplaneConfig config.AirplaneConfig
	hasAirplaneConfig := fsx.Exists(filepath.Join(root, config.FileName))
	if hasAirplaneConfig {
		var err error
		airplaneConfig, err = config.NewAirplaneConfigFromFile(root)
		if err != nil {
			return "", err
		}
	}
	preinstallCommand := airplaneConfig.Python.PreInstall
	postInstallCommand := airplaneConfig.Python.PostInstall

	installHooks, err := GetInstallHooks(entrypoint, root)
	if err != nil {
		return "", err
	}

	baseImageType, _ := opts["base"].(BuildBase)
	useSlimImage := baseImageType == BuildBaseSlim
	v, err := GetVersion(NamePython, "3", useSlimImage)
	if err != nil {
		return "", err
	}

	entrypointFunc, _ := opts["entrypointFunc"].(string)
	shim, err := PythonShim(PythonShimParams{
		TaskRoot:       "/airplane",
		Entrypoint:     entrypoint,
		EntrypointFunc: entrypointFunc,
	})
	if err != nil {
		return "", err
	}

	for i, a := range buildArgs {
		buildArgs[i] = fmt.Sprintf("ARG %s", a)
	}
	argsCommand := strings.Join(buildArgs, "\n")
	dockerfile := heredoc.Doc(`
		FROM {{ .Base }}

		# Install common OS dependencies
		RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
			&& apt-get -y install --no-install-recommends \
				libmemcached-dev \
			&& apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/*

		WORKDIR /airplane
		RUN pip install "airplanesdk>=0.3.0,<0.4.0"
		RUN mkdir -p .airplane && {{.InlineShim}} > .airplane/shim.py

		{{.Args}}

		{{if .PreInstallCommand}}
		RUN {{.PreInstallCommand}}
		{{else if .PreInstallPath}}
		COPY {{ .PreInstallPath }} airplane_preinstall.sh
		RUN chmod +x airplane_preinstall.sh && ./airplane_preinstall.sh
		{{end}}

		{{if .HasRequirements}}
		COPY requirements.txt .
		{{range .EmbeddedRequirements}}
		COPY {{.}} .
		{{end}}
		{{if .HasPipConf}}
		COPY pip.conf .
		ENV PIP_CONFIG_FILE=pip.conf
		{{end}}
		RUN pip install -r requirements.txt
		{{end}}

		{{if .PostInstallCommand}}
		RUN {{.PostInstallCommand}}
		{{else if .PostInstallPath}}
		COPY {{ .PostInstallPath }} airplane_postinstall.sh
		RUN chmod +x airplane_postinstall.sh && ./airplane_postinstall.sh
		{{end}}

		COPY . .
		ENV PYTHONUNBUFFERED=1
		ENTRYPOINT ["python", ".airplane/shim.py"]
	`)

	df, err := applyTemplate(dockerfile, struct {
		Base                 string
		InlineShim           string
		HasRequirements      bool
		EmbeddedRequirements []string
		HasPipConf           bool
		Args                 string
		PreInstallPath       string
		PostInstallPath      string
		PreInstallCommand    string
		PostInstallCommand   string
	}{
		Base:                 v.String(),
		InlineShim:           inlineString(shim),
		HasRequirements:      hasRequirements,
		EmbeddedRequirements: embeddedRequirements,
		HasPipConf:           fsx.Exists(filepath.Join(root, "pip.conf")),
		Args:                 argsCommand,
		PreInstallCommand:    preinstallCommand,
		PostInstallCommand:   postInstallCommand,
		PreInstallPath:       installHooks.PreInstallFilePath,
		PostInstallPath:      installHooks.PostInstallFilePath,
	})
	if err != nil {
		return "", errors.Wrapf(err, "rendering dockerfile")
	}
	return df, nil
}

func collectEmbeddedRequirements(root, requirementsPath string) ([]string, error) {
	var embeddedRequirements []string
	file, err := os.Open(requirementsPath)
	if err != nil {
		return nil, errors.Wrap(err, "opening requirements.txt")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		// Embedded requirements are of the form `-r embedded_requirements.txt`.
		if len(parts) == 2 && parts[0] == "-r" {
			embeddedReqPath := parts[1]
			// Ensure the embedded requirements file exists and is in the root.
			if strings.Contains(embeddedReqPath, "..") {
				return nil, errors.New("embedded requirements may not contain directory traversal elements (`..`)")
			}

			if !fsx.Exists(filepath.Join(root, embeddedReqPath)) {
				return nil, errors.Errorf("embedded requirements file %s does not exist", embeddedReqPath)
			}
			embeddedRequirements = append(embeddedRequirements, embeddedReqPath)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "reading requirements.txt")
	}

	return embeddedRequirements, nil
}

// Python creates a dockerfile for all Python tasks within a task root.
func pythonBundle(
	root string,
	buildContext BuildContext,
	opts KindOptions,
	buildArgs []string,
	filesToDiscover []string,
) (string, error) {
	if opts["shim"] != "true" {
		return pythonLegacy(root, opts)
	}

	requirementsPath := filepath.Join(root, "requirements.txt")
	hasRequirements := fsx.Exists(requirementsPath)
	var embeddedRequirements []string
	var err error
	if hasRequirements {
		embeddedRequirements, err = collectEmbeddedRequirements(root, requirementsPath)
		if err != nil {
			return "", err
		}
	}

	var airplaneConfig config.AirplaneConfig
	hasAirplaneConfig := fsx.Exists(filepath.Join(root, config.FileName))
	if hasAirplaneConfig {
		var err error
		airplaneConfig, err = config.NewAirplaneConfigFromFile(root)
		if err != nil {
			return "", err
		}
	}
	preinstallCommand := airplaneConfig.Python.PreInstall
	postInstallCommand := airplaneConfig.Python.PostInstall

	// Install hooks can only exist in the task root for bundle builds
	installHooks, err := GetInstallHooks("", root)
	if err != nil {
		return "", err
	}

	useSlimImage := buildContext.Base == BuildBaseSlim
	v, err := GetVersion(NamePython, string(buildContext.VersionOrDefault()), useSlimImage)
	if err != nil {
		return "", err
	}

	shim, err := UniversalPythonShim("/airplane")
	if err != nil {
		return "", err
	}

	for i, a := range buildArgs {
		buildArgs[i] = fmt.Sprintf("ARG %s", a)
	}
	argsCommand := strings.Join(buildArgs, "\n")

	// Add build tools.
	buildToolsPath := path.Join(root, ".airplane-build-tools")
	if err := os.MkdirAll(buildToolsPath, 0755); err != nil {
		return "", errors.Wrapf(err, "creating build tools path")
	}
	if len(filesToDiscover) > 0 {
		// Generate parser and store on context
		parserPath := path.Join(buildToolsPath, "inlineParser.py")
		if err := os.WriteFile(parserPath, []byte(parser.PythonParserScript), 0755); err != nil {
			return "", errors.Wrap(err, "writing parser script")
		}
	}
	dockerfile := heredoc.Doc(`
		FROM {{ .Base }}

		# Install common OS dependencies
		RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
			&& apt-get -y install --no-install-recommends \
				libmemcached-dev \
			&& apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/*

		WORKDIR /airplane
		RUN pip install "airplanesdk>=0.3.0,<0.4.0"
		RUN mkdir -p .airplane && {{.InlineShim}} > .airplane/shim.py

		{{.Args}}

		{{if .PreInstallCommand}}
		RUN {{.PreInstallCommand}}
		{{else if .PreInstallPath}}
		COPY {{ .PreInstallPath }} airplane_preinstall.sh
		RUN chmod +x airplane_preinstall.sh && ./airplane_preinstall.sh
		{{end}}

		{{if .HasRequirements}}
		COPY requirements.txt .
		{{range .EmbeddedRequirements}}
		COPY {{.}} .
		{{end}}
		{{if .HasPipConf}}
		COPY pip.conf .
		ENV PIP_CONFIG_FILE=pip.conf
		{{end}}
		RUN pip install -r requirements.txt
		{{end}}

		{{if .PostInstallCommand}}
		RUN {{.PostInstallCommand}}
		{{else if .PostInstallPath}}
		COPY {{ .PostInstallPath }} airplane_postinstall.sh
		RUN chmod +x airplane_postinstall.sh && ./airplane_postinstall.sh
		{{end}}

		COPY . .
		ENV PYTHONUNBUFFERED=1

		{{if .FilesToDiscover}}
		RUN python .airplane-build-tools/inlineParser.py {{.FilesToDiscover}}
		{{end}}
	`)

	df, err := applyTemplate(dockerfile, struct {
		Base                 string
		InlineShim           string
		HasRequirements      bool
		EmbeddedRequirements []string
		HasPipConf           bool
		Args                 string
		PreInstallPath       string
		PostInstallPath      string
		PreInstallCommand    string
		PostInstallCommand   string
		FilesToDiscover      string
	}{
		Base:                 v.String(),
		InlineShim:           inlineString(shim),
		HasRequirements:      hasRequirements,
		EmbeddedRequirements: embeddedRequirements,
		HasPipConf:           fsx.Exists(filepath.Join(root, "pip.conf")),
		Args:                 argsCommand,
		PreInstallPath:       installHooks.PreInstallFilePath,
		PostInstallPath:      installHooks.PostInstallFilePath,
		PreInstallCommand:    preinstallCommand,
		PostInstallCommand:   postInstallCommand,
		FilesToDiscover:      strings.Join(filesToDiscover, " "),
	})
	if err != nil {
		return "", errors.Wrapf(err, "rendering dockerfile")
	}
	return df, nil
}

//go:embed python-shim.py
var pythonShim string

//go:embed universal-python-shim.py
var universalPythonShim string

type PythonShimParams struct {
	TaskRoot       string
	Entrypoint     string
	EntrypointFunc string
}

// PythonShim generates a shim file for running Python tasks.
func PythonShim(params PythonShimParams) (string, error) {
	shim, err := applyTemplate(pythonShim, struct {
		TaskRoot       string
		Entrypoint     string
		EntrypointFunc string
	}{
		TaskRoot:       backslashEscape(params.TaskRoot, `"`),
		Entrypoint:     backslashEscape(params.Entrypoint, `"`),
		EntrypointFunc: backslashEscape(params.EntrypointFunc, `"`),
	})
	if err != nil {
		return "", errors.Wrapf(err, "rendering shim")
	}

	return shim, nil
}

// UniversalPythonShim generates a shim file for running bundled Python tasks.
func UniversalPythonShim(taskRoot string) (string, error) {
	shim, err := applyTemplate(universalPythonShim, struct {
		TaskRoot string
	}{
		TaskRoot: backslashEscape(taskRoot, `"`),
	})
	if err != nil {
		return "", errors.Wrapf(err, "rendering shim")
	}

	return shim, nil
}

// PythonLegacy generates a dockerfile for legacy python support.
func pythonLegacy(root string, args KindOptions) (string, error) {
	var entrypoint, _ = args["entrypoint"].(string)
	var main = filepath.Join(root, entrypoint)
	var reqs = filepath.Join(root, "requirements.txt")

	if err := fsx.AssertExistsAll(main); err != nil {
		return "", err
	}

	t, err := template.New("python").Parse(heredoc.Doc(`
		FROM {{ .Base }}
		WORKDIR /airplane
		{{if not .HasRequirements}}
		RUN echo > requirements.txt
		{{end}}
		COPY . .
		RUN pip install -r requirements.txt
		ENTRYPOINT ["python", "/airplane/{{ .Entrypoint }}"]
	`))
	if err != nil {
		return "", err
	}

	v, err := GetVersion(NamePython, "3", false)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := t.Execute(&buf, struct {
		Base            string
		Entrypoint      string
		HasRequirements bool
	}{
		Base:            v.String(),
		Entrypoint:      entrypoint,
		HasRequirements: fsx.AssertExistsAll(reqs) == nil,
	}); err != nil {
		return "", err
	}

	return buf.String(), nil
}
