package initcmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/testutils"
	"github.com/stretchr/testify/require"
)

func TestInitView(t *testing.T) {
	initcmdDir := "../../../cmd/cli/views/initcmd"
	testCases := []struct {
		desc       string
		req        InitViewRequest
		setup      func(*testing.T, string)
		fixtureDir string
		hasError   bool
	}{
		{
			desc:     "no info",
			hasError: true,
		},
		{
			desc: "View",
			req: InitViewRequest{
				Name: "My view",
			},
			fixtureDir: initcmdDir + "/fixtures/view",
		},
		{
			desc: "View with existing file",
			req: InitViewRequest{
				Name:          "My view",
				suffixCharset: "a",
			},
			setup: func(t *testing.T, wd string) {
				err := os.WriteFile(filepath.Join(wd, "MyView.airplane.tsx"), []byte{}, 0655)
				require.NoError(t, err)
			},
			fixtureDir: "./fixtures/view_with_entrypoint",
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			require := require.New(t)
			ctx := context.Background()

			testutils.TestWithWorkingDirectory(t, test.fixtureDir, func(wd string) bool {
				if test.setup != nil {
					test.setup(t, wd)
				}

				test.req.WorkingDirectory = wd

				_, err := InitView(ctx, test.req)
				if test.hasError {
					require.Error(err)
					return false
				} else {
					require.NoError(err)
					return true
				}
			})
		})
	}
}
