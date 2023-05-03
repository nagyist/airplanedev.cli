from datetime import date, datetime, timezone
from typing import Optional
from typing_extensions import Annotated
import airplane


@airplane.task()
def my_task(
    simple: str,
    default_name: str,
    all: Annotated[
        Optional[str],
        airplane.ParamConfig(
            name="All fields",
            description="My description",
            regex="^.*$",
            options=[
                "Thing 1",
                "Thing 2",
                airplane.LabeledOption(label="Thing 3", value="Secret gremlin"),
            ],
        ),
    ] = "My default",
    shorttext: str = "Text",
    longtext: airplane.LongText = "Longer text",
    sql: airplane.SQL = "SELECT 1",
    boolean_true: bool = True,
    boolean_false: bool = False,
    upload: airplane.File = "upl123",
    integer: int = 10,
    integer_zero: int = 0,
    float: float = 3.14,
    float_zero: float = 0,
    date: date = date(2006, 1, 2),
    datetime: datetime = datetime(2006, 1, 2, 15, 4, 5, tzinfo=timezone.utc),
    datetime2: datetime = datetime(2006, 1, 2, 15, 4, 5, 123000, tzinfo=timezone.utc),
    configvar: airplane.ConfigVar = airplane.ConfigVar("MY_CONFIG"),
    configvar_legacy: airplane.ConfigVar = airplane.ConfigVar("MY_CONFIG"),
    default_name_optional: Optional[str] = None,
):
    pass
