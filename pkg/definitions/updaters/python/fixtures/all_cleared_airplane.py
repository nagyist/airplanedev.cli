import airplane
from datetime import datetime, timezone
from typing import Annotated, Optional


@airplane.task(
    name="Task name",
    description="Task description",
    resources=[airplane.Resource("db")],
    env_vars=[
        airplane.EnvVar(name="CONFIG", config_var_name="aws_access_key"),
        airplane.EnvVar(name="VALUE", value="Hello World!"),
    ],
    timeout=60,
    constraints={"cluster": "k8s", "vpc": "tasks"},
    require_requests=True,
    allow_self_approvals=False,
    restrict_callers=["view", "task"],
    schedules=[
        airplane.Schedule(
            slug="all",
            name="All fields",
            description="A description",
            cron="0 12 * * *",
            param_values={
                "datetime": datetime(2006, 1, 2, 15, 4, 5, tzinfo=timezone.utc)
            },
        ),
        airplane.Schedule(slug="min", cron="* * * * *"),
    ],
)
def my_task(
    dry: Annotated[
        Optional[bool],
        airplane.ParamConfig(
            name="Dry run?", description="Whether or not to run in dry-run mode."
        ),
    ] = True,
    datetime: Optional[datetime] = None,
):
    """
    my_task has a docstring!
    """
    pass
