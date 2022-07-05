package gentypes

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTypeScriptTypes(t *testing.T) {
	fixturesPath, _ := filepath.Abs("./fixtures")
	testCases := []struct {
		desc          string
		file          string
		existingTasks map[string]libapi.Task
		taskSlug      string
	}{
		{
			desc: "task has all param types",
			file: "allparams.ts",
			existingTasks: map[string]libapi.Task{
				"my_task": {
					Slug: "my_task",
					Name: "My Task",
					Kind: build.TaskKindNode,
					Parameters: libapi.Parameters{
						{Slug: "int", Type: libapi.TypeInteger},
						{Slug: "float", Type: libapi.TypeFloat},
						{Slug: "string", Type: libapi.TypeString},
						{Slug: "date", Type: libapi.TypeDate},
						{Slug: "upload", Type: libapi.TypeUpload},
						{Slug: "date-time", Type: libapi.TypeDatetime},
						{Slug: "config_var", Type: libapi.TypeConfigVar},
					},
				},
			},
		},
		{
			desc: "task has no params",
			file: "emptyparams.ts",
			existingTasks: map[string]libapi.Task{
				"my_task": {Slug: "my_task", Name: "My Task", Kind: build.TaskKindNode},
			},
		},
		{
			desc: "multiple tasks",
			file: "multipletasks.ts",
			existingTasks: map[string]libapi.Task{
				"my_task": {Slug: "my_task", Name: "My Task", Kind: build.TaskKindNode, Parameters: libapi.Parameters{
					{Slug: "int", Type: libapi.TypeInteger},
				}},
				"my_task2": {Slug: "my_task2", Name: "My Task 2", Kind: build.TaskKindNode, Parameters: libapi.Parameters{
					{Slug: "string", Type: libapi.TypeString},
				}},
			},
		},
		{
			desc:     "multiple tasks but single task specified",
			file:     "singletask.ts",
			taskSlug: "my_task",
			existingTasks: map[string]libapi.Task{
				"my_task": {Slug: "my_task", Name: "My Task", Kind: build.TaskKindNode, Parameters: libapi.Parameters{
					{Slug: "int", Type: libapi.TypeInteger},
				}},
				"my_task2": {Slug: "my_task2", Name: "My Task 2", Kind: build.TaskKindNode, Parameters: libapi.Parameters{
					{Slug: "string", Type: libapi.TypeString},
				}},
			},
		},
		{
			desc: "non node tasks skipped",
			file: "emptyparams.ts",
			existingTasks: map[string]libapi.Task{
				"my_task":  {Slug: "my_task", Name: "My Task", Kind: build.TaskKindNode},
				"my_task2": {Slug: "my_task2", Name: "My Task2", Kind: build.TaskKindPython},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			var b []byte
			var err error
			if tC.file != "" {
				b, err = os.ReadFile(fixturesPath + "/" + tC.file)
				require.NoError(err)
			}

			client := &api.MockClient{
				Tasks: tC.existingTasks,
			}

			var buff bytes.Buffer
			genTypeScriptTypes(context.Background(), client, "", genTypeScriptTypesOpts{
				taskSlug: tC.taskSlug,
				wr:       &buff,
			})

			assert.Equal(string(b), buff.String())
		})
	}
}
