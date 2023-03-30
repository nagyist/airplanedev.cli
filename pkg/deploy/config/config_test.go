package config

import (
	"path/filepath"
	"testing"

	"github.com/airplanedev/lib/pkg/utils/pointers"
	"github.com/stretchr/testify/require"
)

func TestNewAirplaneConfigFromFile(t *testing.T) {
	fixturesPath, _ := filepath.Abs("./fixtures")
	testCases := []struct {
		desc           string
		fixture        string
		airplaneConfig AirplaneConfig
	}{
		{
			desc:    "yaml with all values",
			fixture: "allvalues/airplane.yaml",
			airplaneConfig: AirplaneConfig{
				Javascript: JavaScriptConfig{
					NodeVersion: "18",
					EnvVars: TaskEnv{
						"fromValue":  EnvVarValue{Value: pointers.String("value")},
						"fromValue2": EnvVarValue{Value: pointers.String("value2")},
						"fromConfig": EnvVarValue{Config: pointers.String("CONFIG")},
					},
					Base:        "slim",
					PreInstall:  "preinstall",
					PostInstall: "postinstall",
				},
				Python: PythonConfig{
					EnvVars: TaskEnv{
						"fromValue":  EnvVarValue{Value: pointers.String("value3")},
						"fromValue2": EnvVarValue{Value: pointers.String("value4")},
						"fromConfig": EnvVarValue{Config: pointers.String("CONFIG2")},
					},
					Version:     "3.11",
					Base:        "full",
					PreInstall:  "preinstall",
					PostInstall: "postinstall",
				},
				View: ViewConfig{
					EnvVars: TaskEnv{
						"fromValue": EnvVarValue{Value: pointers.String("viewValue")},
					},
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)
			path := filepath.Join(fixturesPath, tC.fixture)
			c, err := NewAirplaneConfigFromFile(path)

			require.NoError(err)
			require.Equal(tC.airplaneConfig, c)
		})
	}
}
