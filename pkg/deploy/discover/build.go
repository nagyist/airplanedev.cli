package discover

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/airplanedev/cli/pkg/build/node"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/deploy/discover/parser"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/pkg/errors"
)

var (
	entityConfigExtractionLine = regexp.MustCompile(`.*EXTRACTED_ENTITY_CONFIGS:(.+)`)
)

// esbuildUserFiles builds an airplane entity -> root/.airplane/discover/.
func esbuildUserFiles(log logger.Logger, rootDir, file string) error {
	rootPackageJSON := filepath.Join(rootDir, "package.json")
	hasPackageJSON := fsx.AssertExistsAll(rootPackageJSON) == nil
	packageJSONs, usesWorkspaces, err := node.GetPackageJSONs(rootPackageJSON)
	if err != nil {
		return err
	}
	var externals []string
	if hasPackageJSON {
		// Workaround to get esbuild to not bundle dependencies.
		// See build.ExternalPackages for details.
		externals, err = node.ExternalPackages(packageJSONs, usesWorkspaces)
		if err != nil {
			return err
		}
	}

	res := esbuild.Build(esbuild.BuildOptions{
		EntryPoints: []string{file},
		Outdir:      filepath.Join(rootDir, ".airplane", "discover"),
		Outbase:     rootDir,
		Write:       true,

		Platform: esbuild.PlatformNode,
		Engines: []esbuild.Engine{
			{Name: esbuild.EngineNode, Version: string(buildtypes.DefaultNodeVersion)},
		},
		Format:   esbuild.FormatCommonJS,
		Bundle:   true,
		External: externals,
		Plugins: []esbuild.Plugin{
			{
				Name:  "Remove css",
				Setup: removeCSSEsbuildPlugin,
			},
		},
	})
	var errMsgs []string
	for _, e := range res.Errors {
		msg := e.Text
		if strings.HasPrefix(e.Text, "Could not resolve") {
			msg = fmt.Sprintf("%s. Did you forget to install a dependency?", msg)
		}
		errMsgs = append(errMsgs, msg)
	}

	if len(res.OutputFiles) == 0 {
		msg := "failed to build code"
		if len(errMsgs) > 0 {
			msg = strings.Join(errMsgs, "\n")
		}
		return errors.New(msg)
	}
	return nil
}

// Gets the path of the compiled user file.
func compiledFilePath(rootDir, file string) (string, error) {
	fileAbs, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}
	relPathFromRoot, err := filepath.Rel(rootDir, fileAbs)
	if err != nil {
		return "", errors.New("unable to determine relative path of view from root")
	}
	compiledJSPath := filepath.Join(rootDir, ".airplane", "discover", relPathFromRoot)
	compiledJSPath = strings.TrimSuffix(compiledJSPath, filepath.Ext(compiledJSPath))
	compiledJSPath = compiledJSPath + ".js"
	return compiledJSPath, nil
}

type ParsedJSConfigs struct {
	TaskConfigs []map[string]interface{} `json:"taskConfigs"`
	ViewConfigs []map[string]interface{} `json:"viewConfigs"`
}

// Extracts view and code configs from a compiled JS file.
func extractJSConfigs(file string, env []string) (ParsedJSConfigs, error) {
	tempFile, err := os.CreateTemp("", "airplane.parser.node.*.js")
	if err != nil {
		return ParsedJSConfigs{}, errors.Wrap(err, "creating temporary file")
	}
	defer os.Remove(tempFile.Name())
	_, err = tempFile.Write([]byte(parser.NodeParserScript))
	if err != nil {
		return ParsedJSConfigs{}, errors.Wrap(err, "writing parser script")
	}

	// Run parser on the file
	parserCmd := exec.Command("node", tempFile.Name(), file)
	parserCmd.Env = append(os.Environ(), env...)
	out, err := parserCmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ParsedJSConfigs{}, errors.Wrapf(err, "parsing file=%q: %s", file, ee.Stderr)
		}
		return ParsedJSConfigs{}, errors.Wrapf(err, "parsing file=%q", file)
	}

	// Parser output is EXTRACTED_ENTITY_CONFIGS:{...}
	match := entityConfigExtractionLine.FindStringSubmatch(string(out))
	if len(match) != 2 {
		return ParsedJSConfigs{}, errors.Errorf("could not find EXTRACTED_ENTITY_CONFIGS in parser output: %s", string(out))
	}
	configs := match[1]

	var parsedConfigs ParsedJSConfigs
	if err := json.Unmarshal([]byte(configs), &parsedConfigs); err != nil {
		return ParsedJSConfigs{}, errors.Wrapf(err, "unmarshalling parser output %s", configs)
	}
	return parsedConfigs, nil
}

func extractPythonConfigs(file string, env []string) ([]map[string]interface{}, error) {
	parserCmd := exec.Command("python3", "-c", string(parser.PythonParserScript), file)
	parserCmd.Env = append(os.Environ(), env...)
	out, err := parserCmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return []map[string]interface{}{}, errors.Wrapf(err, "parsing file=%q: %s", file, ee.Stderr)
		}
		return []map[string]interface{}{}, errors.Wrapf(err, "parsing file=%q", file)
	}

	// Parser output is EXTRACTED_ENTITY_CONFIGS:{...}
	match := entityConfigExtractionLine.FindStringSubmatch(string(out))
	if len(match) != 2 {
		return []map[string]interface{}{}, errors.Errorf("could not find EXTRACTED_ENTITY_CONFIGS in parser output: %s", string(out))
	}
	configs := match[1]

	var parsedTasks []map[string]interface{}
	if err := json.Unmarshal([]byte(configs), &parsedTasks); err != nil {
		return nil, errors.Wrapf(err, "unmarshalling parser output %s", configs)
	}
	return parsedTasks, nil
}

// removeCSSEsbuildPlugin is an esbuild plugin that replaces all CSS imports with an empty file.
// Without this plugin, we cannot execute a built file that contains CSS because Node.JS does not
// know how to import and execute CSS files.
func removeCSSEsbuildPlugin(pb esbuild.PluginBuild) {
	pb.OnResolve(esbuild.OnResolveOptions{
		Filter: "\\.css$",
	}, func(ora esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
		return esbuild.OnResolveResult{
			External: false,
			// Rewrite all css imports to a hardcoded path that doesn't actually exist.
			// We will tell esbuild how to load this path in the next step.
			Path: "/empty.css",
		}, nil
	})
	pb.OnLoad(esbuild.OnLoadOptions{
		Filter: "\\.css$",
	}, func(ola esbuild.OnLoadArgs) (esbuild.OnLoadResult, error) {
		// Load all css files with a bit of JS that does nothing.
		return esbuild.OnLoadResult{
			Contents: pointers.String("var foo = 6"),
		}, nil
	})
}
