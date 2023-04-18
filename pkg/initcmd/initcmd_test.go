package initcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitResponse(t *testing.T) {
	ir, err := newInitResponse("/foo/bar")
	require.NoError(t, err)
	require.Equal(t, "/foo/bar", ir.WorkingDirectory)

	// Add a created file.
	ir.AddCreatedFile("/foo/bar/foo.txt")
	require.Equal(t, []string{"foo.txt"}, ir.GetCreatedFiles())
	require.Equal(t, []string{}, ir.GetModifiedFiles())

	// Adding it without the working directory should be a no-op.
	ir.AddCreatedFile("foo.txt")
	require.Equal(t, []string{"foo.txt"}, ir.GetCreatedFiles())
	require.Equal(t, []string{}, ir.GetModifiedFiles())

	// Adding it to the modified list should be a no-op.
	ir.AddModifiedFile("/foo/bar/foo.txt")
	require.Equal(t, []string{"foo.txt"}, ir.GetCreatedFiles())
	require.Equal(t, []string{}, ir.GetModifiedFiles())

	// Add a modified file.
	ir.AddModifiedFile("bar.txt")
	require.Equal(t, []string{"foo.txt"}, ir.GetCreatedFiles())
	require.Equal(t, []string{"bar.txt"}, ir.GetModifiedFiles())

	// Adding it with the working directory should be a no-op.
	ir.AddModifiedFile("/foo/bar/bar.txt")
	require.Equal(t, []string{"foo.txt"}, ir.GetCreatedFiles())
	require.Equal(t, []string{"bar.txt"}, ir.GetModifiedFiles())

	// Adding it as a created field should remove it from the modified file list.
	ir.AddCreatedFile("/foo/bar/bar.txt")
	require.Equal(t, []string{"bar.txt", "foo.txt"}, ir.GetCreatedFiles())
	require.Equal(t, []string{}, ir.GetModifiedFiles())

	// Adding something with a different working directory should leave the directory intact.
	ir.AddCreatedFile("/quz/baz.txt")
	require.Equal(t, []string{"bar.txt", "foo.txt", "/quz/baz.txt"}, ir.GetCreatedFiles())
	require.Equal(t, []string{}, ir.GetModifiedFiles())
}
