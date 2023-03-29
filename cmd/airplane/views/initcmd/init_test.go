package initcmd

import (
	"testing"

	"github.com/airplanedev/cli/cmd/airplane/testutils"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	// TODO(justin, 04152023): re-enable tests once we resolve the flaky yarn install collisions
	t.Skip()

	testCases := []testutils.InitTest{
		{
			Desc:       "View",
			Inputs:     []interface{}{"My view"},
			FixtureDir: "./fixtures/view",
		},
		{
			Desc:       "Noninline",
			Inputs:     []interface{}{"Noninline view"},
			FixtureDir: "./fixtures/noninline",
			Args:       []string{"--inline=false"},
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
