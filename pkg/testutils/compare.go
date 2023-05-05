package testutils

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/build/ignore"
	"github.com/airplanedev/cli/pkg/build/node"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func includeFunc(dir string) func(string, os.FileInfo) (bool, error) {
	return func(filePath string, info os.FileInfo) (bool, error) {
		if !info.IsDir() && filepath.Base(filePath) == "yarn.lock" {
			return false, nil
		}

		defaultInclude, err := ignore.Func(dir)
		if err != nil {
			return false, err
		}

		return defaultInclude(filePath, info)
	}
}

func compareDirectories(
	require *require.Assertions,
	dir1, dir2 string,
	checkEqual func(require *require.Assertions, path1 string, path2 string),
	include func(filePath string, info os.FileInfo) (bool, error),
) {
	compareSubsetOfDirectories(require, dir1, dir2, checkEqual, include)
	compareSubsetOfDirectories(require, dir2, dir1, checkEqual, include)
}

// compareDirectories compares the contents of two directories and returns true if all files in dir1 are in and equal
// to a set of files in dir2.
func compareSubsetOfDirectories(
	require *require.Assertions,
	dir1, dir2 string,
	checkEqual func(require *require.Assertions, path1 string, path2 string),
	include func(filePath string, info os.FileInfo) (bool, error),
) {
	err := filepath.Walk(dir1, func(path1 string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if ok, err := include(path1, info); err != nil {
			return errors.Wrap(err, "checking if file should be included")
		} else if info.IsDir() {
			if ok {
				return nil
			}

			return filepath.SkipDir
		} else if !ok {
			return nil
		}

		// Calculate the corresponding file path in dir2
		relPath, err := filepath.Rel(dir1, path1)
		if err != nil {
			return errors.Wrap(err, "calculating relative path")
		}
		path2 := filepath.Join(dir2, relPath)

		checkEqual(require, path1, path2)

		return nil
	})
	require.NoError(err)
}

// equalWithPackageJSONMajorPinned is a equality check with custom logic for package.json files. We don't check the
// versions in package.json files since we don't pin these dependencies whe initializing tasks and views.
func equalWithPackageJSONMajorPinned(require *require.Assertions, path1, path2 string) {
	buf1, err := os.ReadFile(path1)
	require.NoError(err)

	buf2, err := os.ReadFile(path2)
	require.NoError(err)

	// Custom equality check for package.json files. We don't check the version since they aren't pinned when we
	// initialize JavaScript entities.
	if filepath.Base(path1) == "package.json" {
		comparePackageJSONs(require, buf1, buf2)
	} else {
		require.Equal(string(buf1), string(buf2))
	}
}

func comparePackageJSONs(require *require.Assertions, buf1, buf2 []byte) {
	pkg1 := node.PackageJSON{}
	pkg2 := node.PackageJSON{}

	err := json.Unmarshal(buf1, &pkg1)
	require.NoError(err)

	err = json.Unmarshal(buf2, &pkg2)
	require.NoError(err)

	require.Equal(pkg1.Name, pkg2.Name, "package name should be equal")
	compareDependencies(require, pkg1.Dependencies, pkg2.Dependencies)
	compareDependencies(require, pkg1.DevDependencies, pkg2.DevDependencies)
	compareDependencies(require, pkg1.OptionalDependencies, pkg2.OptionalDependencies)
}

// compareDependencies ensures that the keys in deps1 are a superset of the keys in deps2.
func compareDependencies(require *require.Assertions, deps1, deps2 map[string]string) {
	require.Equal(len(deps1), len(deps2), "number of dependencies should be equal")
	for dep := range deps1 {
		require.Contains(deps2, dep, "dependency should be present in both package.json files")
	}
}

// testDirectoryEmpty ensures that a directory is empty.
func testDirectoryEmpty(
	require *require.Assertions,
	dir string,
	include func(filePath string, info os.FileInfo) (bool, error),
) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if ok, err := include(path, info); err != nil {
			return errors.Wrap(err, "checking if file should be included")
		} else if info.IsDir() {
			if ok {
				return nil
			}

			return filepath.SkipDir
		} else if !ok {
			return nil
		}

		require.True(false, "directory is not empty")

		return nil
	})
	require.NoError(err)
}
