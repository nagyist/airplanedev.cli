import airplane
import os

from mylib import MyGlobalDefaultDateTime


# This is not valid and should not get matched on.
@airplane.task()
class MyTask:
    @airplane.task()
    def run(self):
        pass


@airplane.task(slug="deploy" if os.getenv("AIRPLANE_ENV") == "prod" else "deploy_dev")
def computed_slug():
    pass


@airplane.task(restrict_callers=["task"] if os.getenv("AIRPLANE_ENV") == "prod" else [])
def conditional():
    pass

restrict_callers = ["task"]

@airplane.task(restrict_callers=restrict_callers)
def shared_value():
    pass

MY_DEFAULT_CONSTANT = "foo"

@airplane.task()
def shared_default(val: str = MY_DEFAULT_CONSTANT):
    pass


@airplane.task(name=f"My task ({os.getenv('AIRPLANE_ENV')})")
def template():
    pass


@airplane.task(
    # This template string has no computed expressions, so it's fine.
    name=f"My task",
    # Various value types that should not be flagged as computed:
    a=None,
    c=123,
    d=12.34,
    e="hello",
    f="world",
    i=["hello"],
    j={"jj": "world"},
    k=True,
    l=False,
)
def good():
    pass
