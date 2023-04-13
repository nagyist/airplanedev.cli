package initcmd

import (
	"testing"

	"github.com/airplanedev/cli/cmd/airplane/testutils"
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	testutils.SetupYarn(t)
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
