package testutils

import (
	"os"
	"testing"
)

func Tempdir(t testing.TB) string {
	t.Helper()

	name, err := os.MkdirTemp("", "cli_test")
	if err != nil {
		t.Fatalf("tempdir: %s", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(name)
	})

	return name
}
