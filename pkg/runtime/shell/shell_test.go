package shell

import (
	"os"
	"testing"

	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/stretchr/testify/require"
)

func TestShellRuntime(t *testing.T) {
	require := require.New(t)

	r := Runtime{}
	code, fileMode, err := r.Generate(&runtime.Task{
		URL: "https://app.airplane.dev/t/shell_simple",
	})
	require.NoError(err)
	require.Equal(os.FileMode(0744), fileMode)
	require.Equal(`#!/bin/bash
# Linked to https://app.airplane.dev/t/shell_simple [do not edit this line]

# Params are in environment variables as PARAM_{SLUG}, e.g. PARAM_USER_ID
echo "Printing env for debugging purposes:"
env

data='[{"id": 1, "name": "Gabriel Davis", "role": "Dentist"}, {"id": 2, "name": "Carolyn Garcia", "role": "Sales"}]'
# Show output to users. Documentation: https://docs.airplane.dev/tasks/output#log-output-protocol
echo "airplane_output_set ${data}"
`, string(code))
}
