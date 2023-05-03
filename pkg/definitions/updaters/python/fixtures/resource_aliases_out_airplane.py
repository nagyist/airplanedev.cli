import airplane


@airplane.task(
    resources=[
        airplane.Resource("alias", alias="my_alias"),
        airplane.Resource("no_alias"),
    ]
)
def my_task():
    pass
