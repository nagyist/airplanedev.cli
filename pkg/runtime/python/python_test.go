package python

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/stretchr/testify/require"
)

func TestCheckPythonInstalled(t *testing.T) {
	require := require.New(t)

	// Assumes python3 is installed in test environment...
	err := checkPythonInstalled(context.Background(), &logger.MockLogger{})
	require.NoError(err)
}

func TestInlineMinimal(t *testing.T) {
	require := require.New(t)

	defJSON := `{
        "name": "Inline python full3",
        "slug": "inline_python_full3",
        "resources": null,
        "python": {
          "entrypoint": "test_airplane.py"
        }
      }`

	var def *definitions.Definition
	err := json.Unmarshal([]byte(defJSON), &def)
	require.NoError(err)

	out, _, err := Runtime{}.GenerateInline(def)
	require.NoError(err)
	require.Equal(string(out), `import airplane


@airplane.task(
    slug="inline_python_full3",
    name="Inline python full3",
)
def inline_python_full3():
    data = [
        {"id": 1, "name": "Gabriel Davis", "role": "Dentist"},
        {"id": 2, "name": "Carolyn Garcia", "role": "Sales"},
        {"id": 3, "name": "Frances Hernandez", "role": "Astronaut"},
        {"id": 4, "name": "Melissa Rodriguez", "role": "Engineer"},
        {"id": 5, "name": "Jacob Hall", "role": "Engineer"},
        {"id": 6, "name": "Andrea Lopez", "role": "Astronaut"},
    ]

    # Sort the data in ascending order by name.
    data = sorted(data, key=lambda u: u["name"])

    # You can return data to show output to users.
    # Output documentation: https://docs.airplane.dev/tasks/output
    return data
`)
}

func TestInlineEveryType(t *testing.T) {
	require := require.New(t)

	defJSON := `
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
      }`
	var def *definitions.Definition
	err := json.Unmarshal([]byte(defJSON), &def)
	require.NoError(err)

	out, _, err := Runtime{}.GenerateInline(def)
	require.NoError(err)
	require.Equal(`import datetime
from typing import Optional

import airplane
from typing_extensions import Annotated


@airplane.task(
    slug="inline_python_full2",
    name="Inline python full2",
    description="Tests all parameter types for inline python.",
    require_requests=True,
    allow_self_approvals=False,
    timeout=60,
    constraints={
        "mta": "false",
    },
    resources=[
        airplane.Resource(
            alias="db",
            slug="db",
        ),
        airplane.Resource(
            alias="demo",
            slug="demo_db",
        ),
    ],
    schedules=[
        airplane.Schedule(
            slug="every_midnight",
            cron="0 * * * *",
            param_values={
                "optional_file": None,
                "required_bool": True,
                "required_date": datetime.date(2019, 1, 1),
                "required_datetime": datetime.datetime(2019, 1, 1, 2, 0, 0),
                "required_float": 1.2,
                "required_int": 1,
                "required_long_text": "hey",
                "required_str": "hi",
            },
        ),
        airplane.Schedule(
            slug="every_noon",
            cron="0 * * * *",
            name="At noon",
            description="runs at noon",
            param_values={
                "optional_file": None,
                "required_bool": True,
                "required_date": datetime.date(2019, 1, 1),
                "required_datetime": datetime.datetime(2019, 1, 1, 2, 0, 0),
                "required_float": 1.2,
                "required_int": 1,
                "required_long_text": "hey",
                "required_str": "hi",
            },
        ),
    ],
    env_vars=[
        airplane.EnvVar(
            name="TEST_ENV_CONFIG",
            config_var_name="inline_python_config1",
        ),
        airplane.EnvVar(
            name="TEST_ENV_VALUE",
            value="test value",
        ),
    ],
)
def inline_python_full2(
    required_str: Annotated[
        str,
        airplane.ParamConfig(
            slug="required_str",
            name="Required str",
            description="str,",
        ),
    ],
    required_long_text: Annotated[
        airplane.LongText,
        airplane.ParamConfig(
            slug="required_long_text",
            name="Required long text",
            description="airplane.LongText,",
        ),
    ],
    required_bool: Annotated[
        bool,
        airplane.ParamConfig(
            slug="required_bool",
            name="Required bool",
            description="bool,",
        ),
    ],
    required_int: Annotated[
        int,
        airplane.ParamConfig(
            slug="required_int",
            name="Required int",
            description="int,",
        ),
    ],
    required_float: Annotated[
        float,
        airplane.ParamConfig(
            slug="required_float",
            name="Required float",
            description="float,",
        ),
    ],
    required_date: Annotated[
        datetime.date,
        airplane.ParamConfig(
            slug="required_date",
            name="Required date",
            description="datetime.date,",
        ),
    ],
    required_datetime: Annotated[
        datetime.datetime,
        airplane.ParamConfig(
            slug="required_datetime",
            name="Required datetime",
        ),
    ],
    optional_file: Annotated[
        Optional[airplane.File],
        airplane.ParamConfig(
            slug="optional_file",
            name="Optional file",
        ),
    ],
    optional_str: Annotated[
        Optional[str],
        airplane.ParamConfig(
            slug="optional_str",
            name="Optional str",
        ),
    ],
    optional_long_text: Annotated[
        Optional[airplane.LongText],
        airplane.ParamConfig(
            slug="optional_long_text",
            name="Optional long text",
        ),
    ],
    optional_bool: Annotated[
        Optional[bool],
        airplane.ParamConfig(
            slug="optional_bool",
            name="Optional bool",
        ),
    ],
    optional_int: Annotated[
        Optional[int],
        airplane.ParamConfig(
            slug="optional_int",
            name="Optional int",
        ),
    ],
    optional_float: Annotated[
        Optional[float],
        airplane.ParamConfig(
            slug="optional_float",
            name="Optional float",
        ),
    ],
    optional_date: Annotated[
        Optional[datetime.date],
        airplane.ParamConfig(
            slug="optional_date",
            name="Optional date",
        ),
    ],
    optional_datetime: Annotated[
        Optional[datetime.datetime],
        airplane.ParamConfig(
            slug="optional_datetime",
            name="Optional datetime",
            description="datetime.datetime,",
        ),
    ],
    optional_config_var: Annotated[
        Optional[airplane.ConfigVar],
        airplane.ParamConfig(
            slug="optional_config_var",
            name="Config var constraint name",
            options=[
                airplane.LabeledOption(
                    label="inline_python_config1",
                    value="inline_python_config1",
                ),
                airplane.LabeledOption(
                    label="option 2",
                    value="inline_python_config2",
                ),
            ],
        ),
    ],
    default_str: Annotated[
        Optional[str],
        airplane.ParamConfig(
            slug="default_str",
            name="Default str",
        ),
    ] = "str",
    default_long_text: Annotated[
        Optional[airplane.LongText],
        airplane.ParamConfig(
            slug="default_long_text",
            name="Default long text",
        ),
    ] = "LongText",
    default_bool: Annotated[
        Optional[bool],
        airplane.ParamConfig(
            slug="default_bool",
            name="Default bool",
        ),
    ] = True,
    default_int: Annotated[
        Optional[int],
        airplane.ParamConfig(
            slug="default_int",
            name="Default int",
        ),
    ] = 1,
    default_float: Annotated[
        Optional[float],
        airplane.ParamConfig(
            slug="default_float",
            name="Default float",
        ),
    ] = 1.1,
    default_date: Annotated[
        Optional[datetime.date],
        airplane.ParamConfig(
            slug="default_date",
            name="Default date",
        ),
    ] = datetime.date(2019, 1, 1),
    default_datetime: Annotated[
        Optional[datetime.datetime],
        airplane.ParamConfig(
            slug="default_datetime",
            name="Default datetime",
        ),
    ] = datetime.datetime(2019, 1, 1, 1, 0, 0),
    constraints_str: Annotated[
        Optional[str],
        airplane.ParamConfig(
            slug="constraints_str",
            name="Str constraint name",
            options=[
                airplane.LabeledOption(
                    label="option1",
                    value="option1",
                ),
                airplane.LabeledOption(
                    label="option 2",
                    value="option2",
                ),
            ],
        ),
    ] = "option1",
    constraints_str_regex: Annotated[
        Optional[str],
        airplane.ParamConfig(
            slug="constraints_str_regex",
            name="Str constraint name",
            regex="^option.*$",
        ),
    ] = "option1",
    constraints_long_text: Annotated[
        Optional[airplane.LongText],
        airplane.ParamConfig(
            slug="constraints_long_text",
            name="Long text constraint name",
            options=[
                airplane.LabeledOption(
                    label="option1",
                    value="option1",
                ),
                airplane.LabeledOption(
                    label="option 2",
                    value="option2",
                ),
            ],
        ),
    ] = "option1",
    constraints_int: Annotated[
        Optional[int],
        airplane.ParamConfig(
            slug="constraints_int",
            name="Int constraint name",
            options=[
                airplane.LabeledOption(
                    label="1",
                    value=1,
                ),
                airplane.LabeledOption(
                    label="option 2",
                    value=2,
                ),
            ],
        ),
    ] = 2,
    constraints_float: Annotated[
        Optional[float],
        airplane.ParamConfig(
            slug="constraints_float",
            name="Float constraint name",
            options=[
                airplane.LabeledOption(
                    label="1",
                    value=1,
                ),
                airplane.LabeledOption(
                    label="option 2",
                    value=2.2,
                ),
            ],
        ),
    ] = 2.2,
    constraints_date: Annotated[
        Optional[datetime.date],
        airplane.ParamConfig(
            slug="constraints_date",
            name="date constraint name",
            options=[
                airplane.LabeledOption(
                    label="2019-01-01",
                    value=datetime.date(2019, 1, 1),
                ),
                airplane.LabeledOption(
                    label="option 2",
                    value=datetime.date(2019, 1, 2),
                ),
            ],
        ),
    ] = datetime.date(2019, 1, 1),
    constraints_datetime: Annotated[
        Optional[datetime.datetime],
        airplane.ParamConfig(
            slug="constraints_datetime",
            name="datetime constraint name",
            options=[
                airplane.LabeledOption(
                    label="2019-01-01T01:00:00Z",
                    value=datetime.datetime(2019, 1, 1, 1, 0, 0),
                ),
                airplane.LabeledOption(
                    label="option 2",
                    value=datetime.datetime(2019, 1, 2, 2, 0, 0),
                ),
            ],
        ),
    ] = datetime.datetime(2019, 1, 1, 1, 0, 0),
    constraints_config_var: Annotated[
        Optional[airplane.ConfigVar],
        airplane.ParamConfig(
            slug="constraints_config_var",
            name="Config var constraint name",
            options=[
                airplane.LabeledOption(
                    label="inline_python_config1",
                    value="inline_python_config1",
                ),
                airplane.LabeledOption(
                    label="option 2",
                    value="inline_python_config2",
                ),
            ],
        ),
    ] = "inline_python_config1",
    constraints_config_var_regex: Annotated[
        Optional[airplane.ConfigVar],
        airplane.ParamConfig(
            slug="constraints_config_var_regex",
            name="Config var constraint name regex",
            regex="^inline.*$",
        ),
    ] = "inline_python_config1",
):
    data = [
        {"id": 1, "name": "Gabriel Davis", "role": "Dentist"},
        {"id": 2, "name": "Carolyn Garcia", "role": "Sales"},
        {"id": 3, "name": "Frances Hernandez", "role": "Astronaut"},
        {"id": 4, "name": "Melissa Rodriguez", "role": "Engineer"},
        {"id": 5, "name": "Jacob Hall", "role": "Engineer"},
        {"id": 6, "name": "Andrea Lopez", "role": "Astronaut"},
    ]

    # Sort the data in ascending order by name.
    data = sorted(data, key=lambda u: u["name"])

    # You can return data to show output to users.
    # Output documentation: https://docs.airplane.dev/tasks/output
    return data
`, string(out))
}
