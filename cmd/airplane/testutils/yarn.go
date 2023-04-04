package testutils

import "testing"

// SetupYarn prepares the Yarn testing environment.
func SetupYarn(t *testing.T) {
	// Set the cache folder to a temp directory to avoid potential conflicts
	// when running other yarn tests concurrently.
	t.Setenv("YARN_CACHE_FOLDER", t.TempDir())
}
