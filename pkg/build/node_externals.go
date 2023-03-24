package build

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
)

var (
	esmModules = map[string]bool{
		"node-fetch": true,
		"fetch-blob": true,
		// airplane>=0.2.0 depends on node-fetch
		"airplane":                   true,
		"@airplane/workflow-runtime": true,
		"@airplane/views":            true,
		"lodash-es":                  true,
		"p-limit":                    true,
		"p-retry":                    true,
		"humanize-string":            true,
	}

	installHookScripts = []string{
		"dependencies",
		"preinstall",
		"prepare",
		"prepublish",
		"postinstall",
	}
)

// GetPackageJSONs list all the package.json files that belong to a workspace, or just the root package.json if not
// using workspaces. Also returns a boolean indicating whether workspaces are used.
func GetPackageJSONs(rootPackageJSON string) (pathPackageJSONs []string, usesWorkspaces bool, err error) {
	usesWorkspaces, err = hasWorkspaces(rootPackageJSON)
	if err != nil {
		return nil, false, err
	}
	pathPackageJSONs = []string{rootPackageJSON}
	if usesWorkspaces {
		workspacePackageJSONs, err := findWorkspacePackageJSONs(rootPackageJSON)
		if err != nil {
			return nil, false, err
		}
		for _, j := range workspacePackageJSONs {
			if j != rootPackageJSON {
				pathPackageJSONs = append(pathPackageJSONs, j)
			}
		}
	}

	return pathPackageJSONs, usesWorkspaces, nil
}

// GetPackageCopyCmds gets a set of COPY commands that can be used
// to copy just the package json and yarn files needed for a workspace. This allows
// us to do a yarn or npm install on top of just these, allowing us to cache
// the dependencies across builds.
func GetPackageCopyCmds(baseDir string, pathPackageJSONs []string, dest string) ([]string, error) {
	instructions, err := GetPackageCopyInstructions(baseDir, pathPackageJSONs, dest)
	if err != nil {
		return nil, err
	}

	copyCommands := []string{}
	for _, inst := range instructions {
		copyCommands = append(
			copyCommands,
			fmt.Sprintf(
				"COPY %s %s",
				inst.SrcPath,
				inst.DstPath,
			),
		)
	}

	return copyCommands, nil
}

// GetPackageCopyInstructions gets a set of COPY commands that can be used
// to copy just the package json and yarn files needed for a workspace. This allows
// us to do a yarn or npm install on top of just these, allowing us to cache
// the dependencies across builds.
func GetPackageCopyInstructions(baseDir string, pathPackageJSONs []string, dest string) ([]buildtypes.InstallInstruction, error) {
	srcPaths := map[string]struct{}{}
	copyInstructions := []buildtypes.InstallInstruction{}

	for _, pathPackageJSON := range pathPackageJSONs {
		packageDir := filepath.Dir(pathPackageJSON)

		relPackageDir, err := filepath.Rel(baseDir, packageDir)
		if err != nil {
			return nil, errors.Wrap(err, "generating relative path")
		}
		srcPaths[relPackageDir] = struct{}{}
	}

	for srcPath := range srcPaths {
		destPath := filepath.Join(dest, srcPath)
		// Docker requires that the destination ends with a slash
		if !strings.HasSuffix(destPath, string(filepath.Separator)) {
			destPath = destPath + string(filepath.Separator)
		}

		copyInstructions = append(
			copyInstructions,
			buildtypes.InstallInstruction{
				SrcPath: fmt.Sprintf("%s %s",
					filepath.Join(srcPath, "package*.json"),
					// As long as there's a match for the previous glob,
					// Docker won't complain if there aren't any matches for this one.
					filepath.Join(srcPath, "yarn.*"),
				),
				DstPath: destPath,
			})
	}

	// Sort the results so they're returned in a consistent order (which map
	// iteration doesn't guarantee).
	sort.Slice(copyInstructions, func(a, b int) bool {
		numComponentsA := strings.Count(copyInstructions[a].SrcPath+copyInstructions[a].DstPath, "/")
		numComponentsB := strings.Count(copyInstructions[b].SrcPath+copyInstructions[b].DstPath, "/")

		return (numComponentsA < numComponentsB) ||
			(numComponentsA == numComponentsB &&
				copyInstructions[a].SrcPath < copyInstructions[b].SrcPath)
	})

	return copyInstructions, nil
}

// ExternalPackages reads a list of package.json files and returns all dependencies and dev dependencies. This is used
// as a bit of a workaround for esbuild - we're using esbuild to transform code but don't actually want it to bundle.
// We hit issues when it tries to bundle optional packages (and the packages aren't installed) - for example, pg
// optionally depends on pg-native, and using just pg causes esbuild to bundle pg which bundles pg-native, which errors.
// TODO: replace this with a cleaner esbuild plugin that can mark dependencies as external:
// https://github.com/evanw/esbuild/issues/619#issuecomment-751995294
func ExternalPackages(packageJSONs []string, usesWorkspaces bool) ([]string, error) {
	var deps []string
	for _, pathPackageJSON := range packageJSONs {
		var yarnWorkspacePackages map[string]bool
		var err error
		if usesWorkspaces {
			// If we are in a npm/yarn workspace, we want to bundle all packages in the same
			// workspaces so they are run through esbuild.
			yarnWorkspacePackages, err = getWorkspaceDependencyPackages(pathPackageJSON)
			if err != nil {
				return nil, err
			}
		}

		allDeps, err := ListDependencies(pathPackageJSON)
		if err != nil {
			return nil, err
		}
		for dep := range allDeps {
			// Mark all dependencies as external, except for known ESM-only deps. These deps
			// need to be bundled by esbuild so that esbuild can convert them to CommonJS.
			// As long as these modules don't happen to pull in any optional modules, we should be OK.
			// This is a bandaid until we figure out how to handle ESM without bundling.
			// Also don't mark local yarn workspace packages as external so that they get bundled by esbuild
			// and converted to CommonJS.
			if !esmModules[dep] && !yarnWorkspacePackages[dep] {
				deps = append(deps, dep)
			}
		}
	}

	return deps, nil
}

// ListDependenciesFromPackageJSONs lists all dependencies in a set of `package.json` files.
func ListDependenciesFromPackageJSONs(packageJSONs []string) (map[string]string, error) {
	allDeps := make(map[string]string)
	for _, pathPackageJSON := range packageJSONs {
		deps, err := ListDependencies(pathPackageJSON)
		if err != nil {
			return nil, err
		}

		for k, v := range deps {
			allDeps[k] = v
		}
	}

	return allDeps, nil
}

// ListDependencies lists all dependencies (including dev and optional) and their versions in a `package.json` file.
func ListDependencies(pathPackageJSON string) (map[string]string, error) {
	deps := make(map[string]string)

	d, err := ReadPackageJSON(pathPackageJSON)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// There is no package.json. Treat as having no dependencies.
			return map[string]string{}, nil
		}
		return nil, err
	}

	for k, v := range d.Dependencies {
		deps[k] = v
	}
	for k, v := range d.DevDependencies {
		deps[k] = v
	}
	for k, v := range d.OptionalDependencies {
		deps[k] = v
	}
	return deps, nil
}

// findWorkspacePackageJSONs finds all package.jsons in a workspace. We are assuming that all nested package.jsons are
// part of the workspace. A better solution would involve looking at the workspace tree
// and pulling workspaces from it - this is a shortcut.
func findWorkspacePackageJSONs(rootPackageJSON string) ([]string, error) {
	var pathPackageJSONs []string
	workspaceInfo, err := getYarnWorkspaceInfo(rootPackageJSON)
	if err != nil {
		return nil, err
	}

	packageJSONDir := filepath.Dir(rootPackageJSON)
	for _, info := range workspaceInfo {
		pathPackageJSONs = append(pathPackageJSONs, filepath.Join(packageJSONDir, info.Location, "package.json"))
	}

	return pathPackageJSONs, nil
}

func hasWorkspaces(pathPackageJSON string) (bool, error) {
	var pkg PackageJSON
	buf, err := os.ReadFile(pathPackageJSON)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, errors.Wrapf(err, "node: reading %s", pathPackageJSON)
	}

	if err := json.Unmarshal(buf, &pkg); err != nil {
		return false, errors.Wrapf(err, "parsing %s", pathPackageJSON)
	}
	return len(pkg.Workspaces.Workspaces) > 0, nil
}

func hasInstallHooks(pathPackageJSON string) (bool, error) {
	var pkg PackageJSON
	buf, err := os.ReadFile(pathPackageJSON)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, errors.Wrapf(err, "node: reading %s", pathPackageJSON)
	}

	if err := json.Unmarshal(buf, &pkg); err != nil {
		return false, errors.Wrapf(err, "parsing %s", pathPackageJSON)
	}

	for _, script := range installHookScripts {
		if _, ok := pkg.Scripts[script]; ok {
			return true, nil
		}
	}

	return false, nil
}

func isYarnBerry(pathPackageJSON string) (bool, error) {
	cmd := exec.Command("yarn", "-v")
	cmd.Dir = filepath.Dir(pathPackageJSON)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return false, errors.Wrap(err, string(out))
		}
		return false, errors.Wrap(err, "reading yarn/npm workspaces: Do you have yarn installed?")
	}

	ver, err := semver.ParseTolerant(string(out))
	if err != nil {
		return false, errors.Wrapf(err, "determining yarn version %s", string(out))
	}
	return ver.GE(semver.Version{Major: 2}), nil
}

type yarnWorkspaceInfo struct {
	Name                  string   `json:"name"`
	Location              string   `json:"location"`
	WorkspaceDependencies []string `json:"workspaceDependencies"`
}

// getYarnWorkspaceInfo gets information about a yarn workspace using built in yarn commands.
func getYarnWorkspaceInfo(pathPackageJSON string) ([]yarnWorkspaceInfo, error) {
	yarnBerry, err := isYarnBerry(pathPackageJSON)
	if err != nil {
		return nil, err
	}
	var infos []yarnWorkspaceInfo
	if yarnBerry {
		cmd := exec.Command("yarn", "workspaces", "list", "--json", "-v")
		cmd.Dir = filepath.Dir(pathPackageJSON)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if len(out) > 0 {
				return nil, errors.Wrapf(err, "failed to inspect workspaces for directory %q: %s", cmd.Dir, string(out))
			}
			return nil, errors.Wrap(err, "reading yarn/npm workspaces: Do you have yarn installed?")
		}

		// out will be something like:
		// {"location":".","name":"airplane","workspaceDependencies":[],"mismatchedWorkspaceDependencies":[]}
		// {"location":"examples/1","name":"example1","workspaceDependencies":["lib"],"mismatchedWorkspaceDependencies":[]}
		// {"location":"lib","name":"lib","workspaceDependencies":[],"mismatchedWorkspaceDependencies":[]}

		entries := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, entry := range entries {
			var workspaceInfo yarnWorkspaceInfo
			err = json.Unmarshal([]byte(entry), &workspaceInfo)
			if err != nil {
				return nil, errors.Wrapf(err, "unmarshalling yarn workspace info %s", entry)
			}
			infos = append(infos, workspaceInfo)
		}
		return infos, nil
	} else {
		cmd := exec.Command("yarn", "workspaces", "info")
		cmd.Dir = filepath.Dir(pathPackageJSON)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if len(out) > 0 {
				return nil, errors.Wrapf(err, "failed to inspect workspaces for directory %q: %s", cmd.Dir, string(out))
			}
			return nil, errors.Wrap(err, "reading yarn/npm workspaces: Do you have yarn installed?")
		}

		// out will be something like:
		// yarn workspaces v1.22.17
		// {
		//   "pkg1": {
		//     "location": "pkg1",
		//     "workspaceDependencies": [],
		//     "mismatchedWorkspaceDependencies": []
		//   },
		//   "pkg2": {
		//     "location": "pkg2",
		//     "workspaceDependencies": [
		//       "pkg1"
		//     ],
		//     "mismatchedWorkspaceDependencies": []
		//   }
		// }
		// Done in 0.02s.

		r := regexp.MustCompile(`{[\S\s]+}`)
		workspaceJSON := r.FindString(string(out))
		if workspaceJSON == "" {
			return nil, errors.New(fmt.Sprintf("empty yarn workspace info %s", string(out)))
		}
		var workspaceInfoMap map[string]yarnWorkspaceInfo
		err = json.Unmarshal([]byte(workspaceJSON), &workspaceInfoMap)
		if err != nil {
			return nil, errors.Wrapf(err, "unmarshalling yarn workspace info %s", workspaceJSON)
		}

		for name, workspaceInfo := range workspaceInfoMap {
			workspaceInfo.Name = name
			infos = append(infos, workspaceInfo)
		}
	}
	return infos, nil
}

// getWorkspaceDependencyPackages gets all local workspaces that are depended on by other workspaces.
func getWorkspaceDependencyPackages(pathPackageJSON string) (map[string]bool, error) {
	workspaceInfo, err := getYarnWorkspaceInfo(pathPackageJSON)
	if err != nil {
		return nil, err
	}

	packages := make(map[string]bool)
	for _, info := range workspaceInfo {
		for _, dep := range info.WorkspaceDependencies {
			packages[dep] = true
		}
	}

	return packages, nil
}

type yarnListResults struct {
	Type string `json:"type"`
	Data struct {
		Type  string `json:"type"`
		Trees []struct {
			Name string `json:"name"`
		} `json:"trees"`
	} `json:"data"`
}

// getYarnLockPackageVersion runs yarn to get the specific version of a package.
// If yarn isn't in use or the package isn't found, an error is returned.
func getYarnLockPackageVersion(dir string, packageName string) (string, error) {
	yarnBerry, err := isYarnBerry(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", err
	}
	if yarnBerry {
		cmd := exec.Command("yarn", "info", packageName, "--json", "--name-only")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", errors.Wrap(
				err,
				fmt.Sprintf("getting airplane version via yarn: %s", string(out)),
			)
		}
		var res string
		if err := json.Unmarshal(out, &res); err != nil {
			return "", errors.Wrap(err, "parsing yarn info results")
		}
		// res looks like: airplane@npm:0.2.30
		return strings.Split(res, ":")[1], nil
	}
	cmd := exec.Command(
		"yarn",
		"list",
		"--pattern",
		packageName,
		"--silent",
		"--json",
		"--no-progress",
	)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrap(
			err,
			fmt.Sprintf("getting airplane version via yarn: %s", string(out)),
		)
	}

	// If yarn has warnings (e.g., because of a missing package version), then it will output multiple
	// lines in addition to the "tree" one that we want. Skip over the non-tree lines.
	var treeLine string
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if strings.Contains(line, `"type":"tree"`) {
			treeLine = line
			break
		}
	}

	if treeLine == "" {
		return "", fmt.Errorf("could not find package info in yarn output: %s", string(out))
	}

	results := yarnListResults{}
	if err := json.Unmarshal([]byte(treeLine), &results); err != nil {
		return "", errors.Wrap(err, "parsing yarn list results")
	}

	for _, tree := range results.Data.Trees {
		if strings.HasPrefix(tree.Name, fmt.Sprintf("%s@", packageName)) {
			components := strings.SplitN(tree.Name, "@", 2)
			return components[1], nil
		}
	}

	return "", errors.Errorf("no version found for package %q", packageName)
}

type npmPackageLock struct {
	Packages map[string]struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"packages"`
}

// getNPMLockPackageVersion tries to get the version of a package from a
// package-lock.json file in the argument directory. If the file doesn't exist
// or the package isn't in the file, an error is returned.
func getNPMLockPackageVersion(dir string, packageName string) (string, error) {
	contents, err := os.ReadFile(filepath.Join(dir, "package-lock.json"))
	if err != nil {
		return "", errors.Wrap(err, "reading package-lock.json")
	}

	packageLock := npmPackageLock{}
	if err := json.Unmarshal(contents, &packageLock); err != nil {
		return "", errors.Wrap(err, "parsing package-lock.json")
	}

	// The package name is prefixed by "node_modules"
	packageObj, ok := packageLock.Packages[fmt.Sprintf("node_modules/%s", packageName)]
	if !ok {
		return "", errors.Errorf("no version found for package %q", packageName)
	}
	return packageObj.Version, nil
}

// getLockPackageVersion tries to get as specific a package version as possible
// from the user's bundle. It first tries yarn, then parsing package-lock.json,
// then falls back to the argument fallback version (e.g., from a package.json
// file).
func getLockPackageVersion(
	rootDir string,
	packageName string,
	fallbackVersion string,
) string {
	yarnVersion, err := getYarnLockPackageVersion(rootDir, packageName)
	if err == nil {
		return yarnVersion
	}

	npmVersion, err := getNPMLockPackageVersion(rootDir, packageName)
	if err == nil {
		return npmVersion
	}

	return fallbackVersion
}
