package testutils

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// InitTest specifies a test case for airplane init commands.
type InitTest struct {
	// Desc is a description of the test case.
	Desc string
	// Inputs are the Inputs that will be passed to any prompts, in order.
	Inputs []interface{}
	// Args are any arguments (and flags) that will be passed to the Cobra command.
	Args []string
	// FixtureDir is the directory that the test case should be compared against.
	FixtureDir string
}

// TestCommandAndCompare runs the given command and compares the output to the given fixture directory.
func TestCommandAndCompare(
	t *testing.T,
	cmd *cobra.Command,
	args []string,
	fixtureDir string,
) {
	require := require.New(t)
	cwd, err := os.Getwd()
	require.NoError(err)
	defer func() {
		// Change back to the original directory when the current test case is done.
		err = os.Chdir(cwd)
		require.NoError(err)
	}()

	TestWithWorkingDirectory(
		t,
		fixtureDir,
		func(wd string) bool {
			// Change directories so that the Cobra command runs in a temporary directory instead of ./initcmd
			err = os.Chdir(wd)
			require.NoError(err)

			if args == nil {
				// By default, command is set to os.Args[1:]. We don't want this; instead, we want to pass no args so that we
				// can properly test directives like MaximumNArgs, etc. Setting it to nil does nothing, so we set it to the
				// empty slice.
				cmd.SetArgs([]string{})
			} else {
				cmd.SetArgs(args)
			}
			err = cmd.Execute()
			require.NoError(err)
			return true
		},
	)
}
