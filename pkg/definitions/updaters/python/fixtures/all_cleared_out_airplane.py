import airplane
from datetime import datetime, timezone
from typing import Annotated, Optional


@airplane.task()
def my_task():
    """
    my_task has a docstring!
    """
    pass
