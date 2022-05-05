package deploy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/logger"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
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
				Def: &definitions.Definition_0_3{
					Slug: "my_task",
				},
				Source: discover.ConfigSourceDefn,
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
				Def: &definitions.Definition_0_3{
					Slug: "my_task",
				},
				Source: discover.ConfigSourceScript,
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
				Def: &definitions.Definition_0_3{
					Slug: "my_task",
				},
				Source: discover.ConfigSourceScript,
			},
			expectedNil: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			ctx := context.Background()
			defnDiscoverer := &discover.DefnDiscoverer{
				Client: &api.MockClient{
					Tasks: map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task"}},
				},
				Logger: &logger.MockLogger{},
			}

			tc, err := findDefinitionForScript(ctx, test.cfg, &logger.MockLogger{}, defnDiscoverer, test.taskConfig)
			require.NoError(err)
			if test.expectedNil {
				require.Nil(tc)
			} else {
				require.NotNil(tc)
				assert.Equal(discover.ConfigSourceDefn, tc.Source)
			}
		})
	}
}
