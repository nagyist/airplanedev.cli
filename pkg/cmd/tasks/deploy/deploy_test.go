package deploy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindDefinition(t *testing.T) {
	fixturesPath, _ := filepath.Abs("./fixtures")
	for _, test := range []struct {
		name        string
		cfg         config
		taskConfig  discover.TaskConfig
		expectedNil bool
	}{
		{
			name: "from defn",
			taskConfig: discover.TaskConfig{
				TaskEntrypoint: fixturesPath + "/single_task.js",
				Task: libapi.Task{
					Slug: "my_task",
				},
				From: discover.TaskConfigSourceDefn,
			},
			expectedNil: true,
		},
		{
			name: "defn exists, switch",
			cfg: config{
				assumeYes: true,
			},
			taskConfig: discover.TaskConfig{
				TaskEntrypoint: fixturesPath + "/single_task.js",
				Task: libapi.Task{
					Slug: "my_task",
				},
				From: discover.TaskConfigSourceScript,
			},
			expectedNil: false,
		},
		{
			name: "defn exists, no switch",
			cfg: config{
				assumeNo: true,
			},
			taskConfig: discover.TaskConfig{
				TaskEntrypoint: fixturesPath + "/single_task.js",
				Task: libapi.Task{
					Slug: "my_task",
				},
				From: discover.TaskConfigSourceScript,
			},
			expectedNil: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			ctx := context.Background()
			defnDiscoverer := &discover.DefnDiscoverer{
				Client:    &api.MockClient{},
				AssumeYes: test.cfg.assumeYes,
				AssumeNo:  test.cfg.assumeNo,
			}

			tc, err := findDefinitionForScript(ctx, test.cfg, defnDiscoverer, test.taskConfig)
			require.NoError(err)
			if test.expectedNil {
				require.Nil(tc)
			} else {
				// Assert that slug + From are equal
				require.NotNil(tc)
				assert.Equal(test.taskConfig.Task.Slug, tc.Task.Slug)
				assert.Equal(discover.TaskConfigSourceDefn, tc.From)
			}
		})
	}
}
