"""This is a test that uses multi-byte characters such as 你好世界, ߷, or 👨‍👩‍👧‍👧."""
import airplane
from typing_extensions import Annotated


@decorator(name="你好世界, ߷, or 👨‍👩‍👧‍👧")
@airplane.task(name="你好世界, ߷, or 👨‍👩‍👧‍👧", description="你好世界, ߷, or 👨‍👩‍👧‍👧")
@decorator(name="你好世界, ߷, or 👨‍👩‍👧‍👧")
def my_task(
    name: Annotated[
        str,
        airplane.ParamConfig(
            name="你好世界, ߷, or 👨‍👩‍👧‍👧", description="你好世界, ߷, or 👨‍👩‍👧‍👧"
        ),
    ]
):
    """This is a test that uses multi-byte characters such as 你好世界, ߷, or 👨‍👩‍👧‍👧."""
    pass
