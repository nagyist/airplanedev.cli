import datetime
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
    default_run_permissions="task-participants",
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
