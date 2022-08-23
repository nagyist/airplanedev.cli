package discover

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/utils/logger"
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

const outputDir = ".airplane"

// Finds all user .js, .ts, .jsx, .tsx files and builds them in root/.airplane/ with the
// same directory structure as the user code (so relative imports work properly).
func esbuildUserFiles(rootDir string) error {
	var files []string
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if slices.Contains([]string{outputDir, "node_modules"}, d.Name()) {
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
			{Name: esbuild.EngineNode, Version: build.LatestNodeVersion},
		},
		Format: esbuild.FormatCommonJS,
	})
	for _, e := range res.Errors {
		fmt.Printf("esbuild(error): %v\n", e)
	}

	if len(res.OutputFiles) == 0 {
		return errors.New("esbuild failed to produce output files")
	}
	return nil
}

// Patches node_modules/@airplane/views/package.json in the root directory by adding
// "main": "index.dummy.js" to allow the library to resolve properly when importing
// user code in the node deploy parser.
// index.dummy.js is distributed in @airplane/views and contains dummy importable
// code for all of the components that the library exports. This is so that we don't
// run into any browser code that doesn't run properly on import.
func maybePatchNodeModules(logger logger.Logger, rootDir string) (func(), error) {
	cleanup := func() {}
	pkgJSONPath := path.Join(rootDir, "/node_modules/@airplane/views/package.json")
	if _, err := os.Stat(pkgJSONPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return cleanup, errors.Wrap(err, "unable to determine if node modules need patching")
		}
		return cleanup, nil
	}

	tmpPkgJSONPath := pkgJSONPath + "-real"
	// Rename package.json to package.json-real
	if err := os.Rename(pkgJSONPath, tmpPkgJSONPath); err != nil {
		return cleanup, errors.New("unable to rename @airplane/views/package.json")
	}
	// Defer rename package.json-real back to package.json
	cleanup = func() {
		if err := os.Rename(tmpPkgJSONPath, pkgJSONPath); err != nil {
			logger.Warning("unable to rename %s to %s, you need to install node modules again", tmpPkgJSONPath, pkgJSONPath)
		}
	}

	// Copy over contents to package.json
	oldPkgJSONContent, err := os.ReadFile(tmpPkgJSONPath)
	if err != nil {
		return cleanup, errors.New("unable to read old @airplane/views/package.json")
	}

	// Get old package.json content and add main field
	var oldPkgJSON map[string]interface{}
	if err := json.Unmarshal(oldPkgJSONContent, &oldPkgJSON); err != nil {
		return cleanup, errors.New("unable to unmarshal old @airplane/views/package.json")
	}
	oldPkgJSON["main"] = "./dist/index.dummy.js"

	// Write new package.json content
	newPkgJSON, err := json.MarshalIndent(oldPkgJSON, "", "  ")
	if err != nil {
		return cleanup, errors.New("unable to marshal new @airplane/views/package.json")
	}
	if err := os.WriteFile(pkgJSONPath, newPkgJSON, 0644); err != nil {
		return cleanup, errors.New("unable to write new @airplane/views/package.json")
	}
	return cleanup, nil
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

type ParsedConfigs struct {
	TaskConfigs []map[string]interface{} `json:"taskConfigs"`
	ViewConfigs []map[string]interface{} `json:"viewConfigs"`
}

// Extracts view and code configs from a compiled JS file.
func extractConfigs(file string) (ParsedConfigs, error) {
	tempFile, err := os.CreateTemp("", "airplane.parser.node.*.ts")
	if err != nil {
		return ParsedConfigs{}, errors.Wrap(err, "creating temporary directory")
	}
	defer os.Remove(tempFile.Name())
	_, err = tempFile.Write(nodeParserScript)
	if err != nil {
		return ParsedConfigs{}, errors.Wrap(err, "writing parser script")
	}

	// Run parser on the file
	out, err := exec.Command("npx", "-p", "typescript", "-p", "@types/node", "-p", "tsx",
		"tsx", tempFile.Name(), file).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ParsedConfigs{}, errors.Wrapf(err, "parsing file=%q: %s", file, ee.Stderr)
		}
		return ParsedConfigs{}, errors.Wrapf(err, "parsing file=%q", file)
	}

	var parsedConfigs ParsedConfigs
	if err := json.Unmarshal(out, &parsedConfigs); err != nil {
		return ParsedConfigs{}, errors.Wrap(err, "unmarshalling parser output")
	}
	return parsedConfigs, nil
}
