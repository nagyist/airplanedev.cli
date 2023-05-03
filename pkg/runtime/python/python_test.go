package python

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/stretchr/testify/require"
)

func TestCheckPythonInstalled(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Assumes python3 is installed in test environment...
	bin, err := utils.GetPythonBinary(context.Background(), &logger.MockLogger{})
	require.NoError(err)
	require.NotEmpty(bin)
}

func TestGenerateInline(t *testing.T) {
	testCases := []struct {
		desc            string
		defJSON         string
		expectedFixture string
	}{
		{
			desc: "simple",
			defJSON: `{
        "name": "Inline python full3",
        "slug": "inline_python_full3",
        "resources": null,
        "python": {
          "entrypoint": "test_airplane.py"
        }
      }`,
			expectedFixture: "simple.py",
		},
		{
			desc: "simple",
			defJSON: `
      {
          "name": "Inline python full2",
          "slug": "inline_python_full2",
          "description": "Tests all parameter types for inline python.",
          "parameters": [
            {
              "name": "Required str",
              "slug": "required_str",
              "type": "shorttext",
              "description": "str,",
              "required": true
            },
            {
              "name": "Required long text",
              "slug": "required_long_text",
              "type": "longtext",
              "description": "airplane.LongText,",
              "required": true
            },
            {
              "name": "Required bool",
              "slug": "required_bool",
              "type": "boolean",
              "description": "bool,",
              "required": true
            },
            {
              "name": "Required int",
              "slug": "required_int",
              "type": "integer",
              "description": "int,",
              "required": true
            },
            {
              "name": "Required float",
              "slug": "required_float",
              "type": "float",
              "description": "float,",
              "required": true
            },
            {
              "name": "Required date",
              "slug": "required_date",
              "type": "date",
              "description": "datetime.date,",
              "required": true
            },
            {
              "name": "Required datetime",
              "slug": "required_datetime",
              "type": "datetime",
              "required": true
            },
            {
              "name": "Optional file",
              "slug": "optional_file",
              "type": "upload",
              "required": false
            },
            {
              "name": "Optional str",
              "slug": "optional_str",
              "type": "shorttext",
              "required": false
            },
            {
              "name": "Optional long text",
              "slug": "optional_long_text",
              "type": "longtext",
              "required": false
            },
            {
              "name": "Optional bool",
              "slug": "optional_bool",
              "type": "boolean",
              "required": false
            },
            {
              "name": "Optional int",
              "slug": "optional_int",
              "type": "integer",
              "required": false
            },
            {
              "name": "Optional float",
              "slug": "optional_float",
              "type": "float",
              "required": false
            },
            {
              "name": "Optional date",
              "slug": "optional_date",
              "type": "date",
              "required": false
            },
            {
              "name": "Optional datetime",
              "slug": "optional_datetime",
              "type": "datetime",
              "description": "datetime.datetime,",
              "required": false
            },
            {
              "name": "Config var constraint name",
              "slug": "optional_config_var",
              "type": "configvar",
              "required": false,
              "options": [
                {
                  "label": "inline_python_config1",
                  "value": "inline_python_config1"
                },
                {
                  "label": "option 2",
                  "value": "inline_python_config2"
                }
              ]
            },
            {
              "name": "Default str",
              "slug": "default_str",
              "type": "shorttext",
              "default": "str",
              "required": false
            },
            {
              "name": "Default long text",
              "slug": "default_long_text",
              "type": "longtext",
              "default": "LongText",
              "required": false
            },
            {
              "name": "Default bool",
              "slug": "default_bool",
              "type": "boolean",
              "default": true,
              "required": false
            },
            {
              "name": "Default int",
              "slug": "default_int",
              "type": "integer",
              "default": 1,
              "required": false
            },
            {
              "name": "Default float",
              "slug": "default_float",
              "type": "float",
              "default": 1.1,
              "required": false
            },
            {
              "name": "Default date",
              "slug": "default_date",
              "type": "date",
              "default": "2019-01-01",
              "required": false
            },
            {
              "name": "Default datetime",
              "slug": "default_datetime",
              "type": "datetime",
              "default": "2019-01-01T01:00:00Z",
              "required": false
            },
            {
              "name": "Str constraint name",
              "slug": "constraints_str",
              "type": "shorttext",
              "default": "option1",
              "required": false,
              "options": [
                {
                  "label": "option1",
                  "value": "option1"
                },
                {
                  "label": "option 2",
                  "value": "option2"
                }
              ]
            },
            {
              "name": "Str constraint name",
              "slug": "constraints_str_regex",
              "type": "shorttext",
              "default": "option1",
              "required": false,
              "regex": "^option.*$"
            },
            {
              "name": "Long text constraint name",
              "slug": "constraints_long_text",
              "type": "longtext",
              "default": "option1",
              "required": false,
              "options": [
                {
                  "label": "option1",
                  "value": "option1"
                },
                {
                  "label": "option 2",
                  "value": "option2"
                }
              ]
            },
            {
              "name": "Int constraint name",
              "slug": "constraints_int",
              "type": "integer",
              "default": 2,
              "required": false,
              "options": [
                {
                  "label": "1",
                  "value": 1
                },
                {
                  "label": "option 2",
                  "value": 2
                }
              ]
            },
            {
              "name": "Float constraint name",
              "slug": "constraints_float",
              "type": "float",
              "default": 2.2,
              "required": false,
              "options": [
                {
                  "label": "1",
                  "value": 1
                },
                {
                  "label": "option 2",
                  "value": 2.2
                }
              ]
            },
            {
              "name": "date constraint name",
              "slug": "constraints_date",
              "type": "date",
              "default": "2019-01-01",
              "required": false,
              "options": [
                {
                  "label": "2019-01-01",
                  "value": "2019-01-01"
                },
                {
                  "label": "option 2",
                  "value": "2019-01-02"
                }
              ]
            },
            {
              "name": "datetime constraint name",
              "slug": "constraints_datetime",
              "type": "datetime",
              "default": "2019-01-01T01:00:00Z",
              "required": false,
              "options": [
                {
                  "label": "2019-01-01T01:00:00Z",
                  "value": "2019-01-01T01:00:00Z"
                },
                {
                  "label": "option 2",
                  "value": "2019-01-02T02:00:00Z"
                }
              ]
            },
            {
              "name": "Config var constraint name",
              "slug": "constraints_config_var",
              "type": "configvar",
              "default": "inline_python_config1",
              "required": false,
              "options": [
                {
                  "label": "inline_python_config1",
                  "value": "inline_python_config1"
                },
                {
                  "label": "option 2",
                  "value": "inline_python_config2"
                }
              ]
            },
            {
              "name": "Config var constraint name regex",
              "slug": "constraints_config_var_regex",
              "type": "configvar",
              "default": "inline_python_config1",
              "required": false,
              "regex": "^inline.*$"
            }
          ],
          "resources": {
            "db": "db",
            "demo": "demo_db"
          },
          "python": {
            "entrypoint": "test_airplane.py",
            "envVars": {
              "TEST_ENV_CONFIG": {
                "config": "inline_python_config1"
              },
              "TEST_ENV_VALUE": {
                "value": "test value"
              }
            }
          },
          "constraints": {
            "mta": "false"
          },
          "allowSelfApprovals": false,
          "requireRequests": true,
          "timeout": 60,
          "defaultRunPermissions": "task-participants",
          "schedules": {
            "every_midnight": {
              "cron": "0 * * * *",
              "paramValues": {
                "optional_file": null,
                "required_bool": true,
                "required_date": "2019-01-01",
                "required_datetime": "2019-01-01T02:00:00Z",
                "required_float": 1.2,
                "required_int": 1,
                "required_long_text": "hey",
                "required_str": "hi"
              }
            },
            "every_noon": {
              "name": "At noon",
              "description": "runs at noon",
              "cron": "0 * * * *",
              "paramValues": {
                "optional_file": null,
                "required_bool": true,
                "required_date": "2019-01-01",
                "required_datetime": "2019-01-01T02:00:00Z",
                "required_float": 1.2,
                "required_int": 1,
                "required_long_text": "hey",
                "required_str": "hi"
              }
            }
          }
        }`,
			expectedFixture: "every_type.py",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)

			var def *definitions.Definition
			err := json.Unmarshal([]byte(tC.defJSON), &def)
			require.NoError(err)

			out, _, err := Runtime{}.GenerateInline(def)
			require.NoError(err)

			fixtureString, err := os.ReadFile(fmt.Sprintf("./fixtures/generate/%s", tC.expectedFixture))
			require.NoError(err)

			require.Equal(string(fixtureString), string(out))
		})
	}
}
