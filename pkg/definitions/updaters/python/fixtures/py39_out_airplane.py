from typing import Annotated, Optional
import airplane


@airplane.task()
def my_task(
    name: Annotated[Optional[str], airplane.ParamConfig(name="User name")] = None
):
    pass
