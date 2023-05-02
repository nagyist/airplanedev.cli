package definitions

import (
	"os"
	"path/filepath"
	"testing"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/stretchr/testify/require"
)

func TestYAMLComments(t *testing.T) {
	fixturesPath, _ := filepath.Abs("./fixtures")
	for _, test := range []struct {
		descriptor string
		name       string
		slug       string
		file       string
		kind       buildtypes.TaskKind
		entrypoint string
	}{
		{
			descriptor: "python",
			name:       "My Task",
			slug:       "my_task",
			file:       fixturesPath + "/python.task.yaml",
			kind:       buildtypes.TaskKindPython,
			entrypoint: "my_task.py",
		},
		{
			descriptor: "node",
			name:       "My Task",
			slug:       "my_task",
			file:       fixturesPath + "/node.task.yaml",
			kind:       buildtypes.TaskKindNode,
			entrypoint: "my_task.ts",
		},
		{
			descriptor: "shell",
			name:       "My Task",
			slug:       "my_task",
			file:       fixturesPath + "/shell.task.yaml",
			kind:       buildtypes.TaskKindShell,
			entrypoint: "my_task.sh",
		},
		{
			descriptor: "docker",
			name:       "My Task",
			slug:       "my_task",
			file:       fixturesPath + "/docker.task.yaml",
			kind:       buildtypes.TaskKindImage,
		},
		{
			descriptor: "sql",
			name:       "My Task",
			slug:       "my_task",
			file:       fixturesPath + "/sql.task.yaml",
			kind:       buildtypes.TaskKindSQL,
			entrypoint: "my_task.sql",
		},
		{
			descriptor: "rest",
			name:       "My Task",
			slug:       "my_task",
			file:       fixturesPath + "/rest.task.yaml",
			kind:       buildtypes.TaskKindREST,
		},
		{
			descriptor: "name with special characters",
			name:       "[Test] My Task",
			slug:       "test_my_task",
			file:       fixturesPath + "/specialchars.task.yaml",
			kind:       buildtypes.TaskKindREST,
		},
	} {
		t.Run(test.descriptor, func(t *testing.T) {
			require := require.New(t)
			def, err := NewDefinition(test.name, test.slug, test.kind, test.entrypoint)
			require.NoError(err)

			got, err := def.GenerateCommentedFile(DefFormatYAML)
			require.NoError(err)

			expected, err := os.ReadFile(test.file)
			require.NoError(err)

			require.Equal(string(expected), string(got))

			unmarshalled := Definition{}
			err = unmarshalled.Unmarshal(DefFormatYAML, got)
			require.NoError(err)
			require.Equal(def, unmarshalled)
		})
	}
}
