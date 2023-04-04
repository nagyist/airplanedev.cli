package initcmd

import (
	"testing"

	"github.com/airplanedev/cli/cmd/airplane/testutils"
	"github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/stretchr/testify/require"
)

func TestFindTemplate(t *testing.T) {
	testCases := []struct {
		desc             string
		templates        []Template
		gitPath          string
		expectedTemplate Template
	}{
		{
			desc:    "from full URL",
			gitPath: "https://github.com/airplanedev/templates/myTemplate",
			templates: []Template{
				{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
			},
			expectedTemplate: Template{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
		},
		{
			desc:    "from short URL",
			gitPath: "github.com/airplanedev/templates/myTemplate",
			templates: []Template{
				{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
			},
			expectedTemplate: Template{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
		},
		{
			desc:    "from template name",
			gitPath: "myTemplate",
			templates: []Template{
				{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
			},
			expectedTemplate: Template{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			temp, err := FindTemplate(tC.templates, tC.gitPath)
			require.NoError(t, err)
			require.Equal(t, tC.expectedTemplate, temp)
		})
	}
}

func TestInit(t *testing.T) {
	t.Setenv("YARN_CACHE_FOLDER", t.TempDir())
	testCases := []testutils.InitTest{
		{
			Desc:       "Task",
			Inputs:     []interface{}{taskOption, "My task", "JavaScript", "my_task.airplane.ts"},
			FixtureDir: "./fixtures/task",
		},
		{
			Desc:       "View",
			Inputs:     []interface{}{viewOption, "My view"},
			FixtureDir: "./fixtures/view",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.Desc, func(t *testing.T) {
			subR := require.New(t)
			var cfg = &cli.Config{
				Client:   api.NewMockClient(),
				Prompter: prompts.NewMock(tC.Inputs...),
			}

			cmd := New(cfg)
			testutils.TestCommandAndCompare(subR, cmd, tC.Args, tC.FixtureDir)
		})
	}
}
