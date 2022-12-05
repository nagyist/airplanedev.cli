package initcmd

import (
	"bytes"
	"testing"

	"github.com/airplanedev/lib/pkg/build"
	deployconfig "github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/stretchr/testify/require"
)

func TestGetNewAirplaneConfig(t *testing.T) {
	testCases := []struct {
		desc              string
		cfg               deployconfig.AirplaneConfig
		existingConfig    deployconfig.AirplaneConfig
		hasExistingConfig bool
		newConfig         *deployconfig.AirplaneConfig
	}{
		{
			desc:      "Creates new empty config",
			newConfig: &deployconfig.AirplaneConfig{},
		},
		{
			desc: "Creates new config with node version and base",
			cfg: deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(build.BuildTypeVersionNode18),
					Base:        string(build.BuildBaseSlim),
				},
			},
			newConfig: &deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(build.BuildTypeVersionNode18),
					Base:        string(build.BuildBaseSlim),
				},
			},
		},
		{
			desc: "Does not update a non-empty config",
			cfg: deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(build.BuildTypeVersionNode18),
					Base:        string(build.BuildBaseSlim),
				},
			},
			hasExistingConfig: true,
			existingConfig: deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(build.BuildTypeVersionNode18),
					Base:        string(build.BuildBaseSlim),
				},
			},
		},
		{
			desc: "Updates existing, empty config",
			cfg: deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(build.BuildTypeVersionNode18),
				},
			},
			hasExistingConfig: true,
			newConfig: &deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(build.BuildTypeVersionNode18),
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			var w bytes.Buffer

			err := writeNewAirplaneConfig(&w, getNewAirplaneConfigOptions{
				cfg:               tC.cfg,
				existingConfig:    tC.existingConfig,
				hasExistingConfig: tC.hasExistingConfig,
			})
			require.NoError(t, err)

			c := &deployconfig.AirplaneConfig{}
			err = c.Unmarshal(w.Bytes())
			require.NoError(t, err)

			if w.Len() == 0 {
				require.Nil(t, tC.newConfig)
			} else {
				require.NotNil(t, tC.newConfig)
				require.Equal(t, *tC.newConfig, *c)
			}
		})
	}
}
