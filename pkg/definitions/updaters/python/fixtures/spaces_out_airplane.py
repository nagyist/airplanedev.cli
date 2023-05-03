from typing import Optional
from typing_extensions import Annotated
import airplane


@airplane.task(
  name="Task name",
  env_vars=[
    airplane.EnvVar(name="CONFIG", config_var_name="aws_access_key"),
    airplane.EnvVar(name="VALUE", value="Hello World!"),
  ],
)
def my_task(
  dry: Annotated[
    Optional[bool],
    airplane.ParamConfig(
      name="Dry run?", description="Whether or not to run in dry-run mode."
    ),
  ] = True
):
  pass
