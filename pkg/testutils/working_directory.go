package testutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithWorkingDirectory(
	t *testing.T,
	fixtureDir string,
	work func(string) bool,
) {
	t.Setenv("YARN_CACHE_FOLDER", t.TempDir())
	require := require.New(t)
	testIsEmpty := fixtureDir == ""
	if !testIsEmpty {
		absFixtureDir, err := filepath.Abs(fixtureDir)
		require.NoError(err)
		fixtureDir = absFixtureDir
	} else {
		tmpFixtureDir, err := os.MkdirTemp("", "empty_test")
		require.NoError(err)
		fixtureDir = tmpFixtureDir
		defer os.RemoveAll(tmpFixtureDir)
	}

	// Create a temporary directory and cd into it
	tempDir, err := os.MkdirTemp("", "cli_test_*")
	require.NoError(err)
	defer os.RemoveAll(tempDir)

	// The name of the directory that a cobra command is run in may affect the output of the command. As such, we create
	// a subdirectory with the base name of the fixture directory so that the output of the command is consistent.
	subdir := filepath.Base(fixtureDir)
	tempDir = filepath.Join(tempDir, subdir)
	err = os.MkdirAll(tempDir, 0755)
	require.NoError(err)

	doCheck := work(tempDir)

	if doCheck {
		include := includeFunc(fixtureDir)
		if testIsEmpty {
			testDirectoryEmpty(require, tempDir, include)
		} else {
			compareDirectories(require, fixtureDir, tempDir, equalWithPackageJSONMajorPinned, include)
		}
	}
}
