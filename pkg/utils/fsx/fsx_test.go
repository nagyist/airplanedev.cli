package fsx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFind(t *testing.T) {
	t.Run("task with package.json", func(t *testing.T) {
		var assert = require.New(t)
		var path = "testdata/my/task/with_package_json"
		var filename = "package.json"

		v, ok := Find(path, filename)

		assert.True(ok)
		assert.Equal("with_package_json", filepath.Base(v))
	})

	t.Run("task within monorepo", func(t *testing.T) {
		var assert = require.New(t)
		var path = "testdata/monorepo/my/task"
		var filename = "package.json"

		v, ok := Find(path, filename)

		assert.True(ok)
		assert.Equal("monorepo", filepath.Base(v))
	})

	t.Run("missing package.json", func(t *testing.T) {
		var assert = require.New(t)
		var path = "testdata"
		var filename = "package.json"

		v, ok := Find(path, filename)

		assert.False(ok)
		assert.Equal("", v)
	})
}

func TestFindUntil(t *testing.T) {
	var assert = require.New(t)

	getFile := func(p string) string {
		c, err := os.ReadFile(p)
		assert.NoError(err)
		return strings.TrimSpace(string(c))
	}

	// Should still search `c`:
	filename := "c.txt"
	v, ok := FindUntil("testdata/a/b/c", "testdata/a/b/c", filename)
	assert.True(ok)
	assert.Equal("c", getFile(filepath.Join(v, filename)))

	// Should return the lowest file:
	v, ok = FindUntil("testdata/a/b/c", "testdata", filename)
	assert.True(ok)
	assert.Equal("c", getFile(filepath.Join(v, filename)))

	// Should find files in other directories:
	v, ok = FindUntil("testdata/a/b", "testdata", filename)
	assert.True(ok)
	assert.Equal("b", getFile(filepath.Join(v, filename)))

	// Should traverse up a directory:
	filename = "b.txt"
	v, ok = FindUntil("testdata/a/b/c", "testdata", filename)
	assert.True(ok)
	assert.Equal("b", getFile(filepath.Join(v, filename)))
}

func TestAssertExistsAll(t *testing.T) {
	var assert = require.New(t)

	cwd, err := os.Getwd()
	assert.NoError(err)

	err = AssertExistsAll(
		cwd+"/testdata/a/b/a.txt",
		cwd+"/testdata/a/b/b.txt",
		cwd+"/testdata/a/b/c.txt",
	)
	assert.ErrorContains(err, "could not find file")

	err = AssertExistsAll(
		cwd+"/testdata/a/b/b.txt",
		cwd+"/testdata/a/b/c.txt",
	)
	assert.NoError(err)
}

func TestAssertExistsAny(t *testing.T) {
	var assert = require.New(t)

	cwd, err := os.Getwd()
	assert.NoError(err)

	err = AssertExistsAny(
		cwd+"/testdata/monorepo/my/task/activities.ts",
		cwd+"/testdata/monorepo/my/task/activities.js",
	)
	assert.ErrorContains(err, "could not find any files")

	err = AssertExistsAny(cwd + "/testdata/monorepo/my/task/task.ts")
	assert.NoError(err)
}
