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
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

const outputDir = ".airplane"

// Finds all user .js, .ts, .jsx, .tsx files and builds them in root/.airplane/ with the
// same directory structure as the user code (so relative imports work properly).
func esbuildUserFiles(rootDir string) error {
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
		Outdir:      filepath.Join(rootDir, ".airplane"),
		Outbase:     rootDir,
		Write:       true,

		Platform: esbuild.PlatformNode,
		Engines: []esbuild.Engine{
			{Name: esbuild.EngineNode, Version: build.DefaultNodeVersion},
		},
		Format:   esbuild.FormatCommonJS,
		Bundle:   true,
		External: externals,
	})
	for _, e := range res.Errors {
		fmt.Printf("esbuild(error): %v\n", e)
	}

	if len(res.OutputFiles) == 0 {
		return errors.New("esbuild failed to produce output files")
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
	compiledJSPath := filepath.Join(rootDir, ".airplane", relPathFromRoot)
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
		return ParsedJSConfigs{}, errors.Wrap(err, "creating temporary directory")
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

	var parsedConfigs ParsedJSConfigs
	if err := json.Unmarshal(out, &parsedConfigs); err != nil {
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
	var parsedTasks []map[string]interface{}
	if err := json.Unmarshal(out, &parsedTasks); err != nil {
		return nil, errors.Wrap(err, "unmarshalling parser output")
	}
	return parsedTasks, nil
}
