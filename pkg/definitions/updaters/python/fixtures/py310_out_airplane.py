from typing import Annotated
import airplane


@airplane.task()
def my_task(name: Annotated[str | None, airplane.ParamConfig(name="User name")] = None):
    pass
