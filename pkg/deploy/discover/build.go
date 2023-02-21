package discover

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/airplanedev/lib/pkg/utils/pointers"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

const outputDir = ".airplane"

// Finds all user .js, .ts, .jsx, .tsx files and builds them in root/.airplane/ with the
// same directory structure as the user code (so relative imports work properly).
func esbuildUserFiles(log logger.Logger, rootDir string) error {
	rootPackageJSON := filepath.Join(rootDir, "package.json")
	hasPackageJSON := fsx.AssertExistsAll(rootPackageJSON) == nil
	packageJSONs, usesWorkspaces, err := build.GetPackageJSONs(rootPackageJSON)
	if err != nil {
		return err
	}
	var externals []string
	if hasPackageJSON {
		// Workaround to get esbuild to not bundle dependencies.
		// See build.ExternalPackages for details.
		externals, err = build.ExternalPackages(packageJSONs, usesWorkspaces)
		if err != nil {
			return err
		}
	}

	var files []string
	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if slices.Contains([]string{outputDir, "node_modules", ".airplane-view"}, d.Name()) {
			return filepath.SkipDir
		}
		if slices.Contains([]string{".js", ".ts", ".jsx", ".tsx"}, filepath.Ext(d.Name())) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New(fmt.Sprintf("unable to find any user js/ts/jsx/tsx files in %s", rootDir))
	}

	res := esbuild.Build(esbuild.BuildOptions{
		EntryPoints: files,
		Outdir:      filepath.Join(rootDir, ".airplane", "discover"),
		Outbase:     rootDir,
		Write:       true,

		Platform: esbuild.PlatformNode,
		Engines: []esbuild.Engine{
			{Name: esbuild.EngineNode, Version: string(build.DefaultNodeVersion)},
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
func extractJSConfigs(file string) (ParsedJSConfigs, error) {
	tempFile, err := os.CreateTemp("", "airplane.parser.node.*.js")
	if err != nil {
		return ParsedJSConfigs{}, errors.Wrap(err, "creating temporary file")
	}
	defer os.Remove(tempFile.Name())
	_, err = tempFile.Write(nodeParserScript)
	if err != nil {
		return ParsedJSConfigs{}, errors.Wrap(err, "writing parser script")
	}

	// Run parser on the file
	out, err := exec.Command("node", tempFile.Name(), file).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ParsedJSConfigs{}, errors.Wrapf(err, "parsing file=%q: %s", file, ee.Stderr)
		}
		return ParsedJSConfigs{}, errors.Wrapf(err, "parsing file=%q", file)
	}

	// Parser output is EXTRACTED_ENTITY_CONFIGS:{...}
	parsedOutput := strings.SplitN(string(out), ":", 2)

	var parsedConfigs ParsedJSConfigs
	if err := json.Unmarshal([]byte(parsedOutput[1]), &parsedConfigs); err != nil {
		return ParsedJSConfigs{}, errors.Wrap(err, "unmarshalling parser output")
	}
	return parsedConfigs, nil
}

func extractPythonConfigs(file string) ([]map[string]interface{}, error) {
	out, err := exec.Command("python3", "-c", string(pythonParserScript), file).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return []map[string]interface{}{}, errors.Wrapf(err, "parsing file=%q: %s", file, ee.Stderr)
		}
		return []map[string]interface{}{}, errors.Wrapf(err, "parsing file=%q", file)
	}

	// Parser output is EXTRACTED_ENTITY_CONFIGS:{...}
	parsedOutput := strings.SplitN(string(out), ":", 2)

	var parsedTasks []map[string]interface{}
	if err := json.Unmarshal([]byte(parsedOutput[1]), &parsedTasks); err != nil {
		return nil, errors.Wrap(err, "unmarshalling parser output")
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
