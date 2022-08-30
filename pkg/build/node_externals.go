package build

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
)

var esmModules = map[string]bool{
	"node-fetch": true,
	// airplane>=0.2.0 depends on node-fetch
	"airplane":        true,
	"@airplane/views": true,
}

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

	f, err := os.Open(pathPackageJSON)
	if err != nil {
		// There is no package.json (or we can't open it). Treat as having no dependencies.
		return map[string]string{}, nil
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, errors.Wrap(err, "reading package.json")
	}
	var d struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, errors.Wrap(err, "unmarshaling package.json")
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
	if _, err := os.Stat(pathPackageJSON); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	var pkg pkgJSON
	buf, err := os.ReadFile(pathPackageJSON)
	if err != nil {
		return false, errors.Wrapf(err, "node: reading %s", pathPackageJSON)
	}

	if err := json.Unmarshal(buf, &pkg); err != nil {
		return false, errors.Wrapf(err, "parsing %s", pathPackageJSON)
	}
	return len(pkg.Workspaces.workspaces) > 0, nil
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
