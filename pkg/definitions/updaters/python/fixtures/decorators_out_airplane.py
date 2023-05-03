import airplane


@decorator()
@airplane.task(description="Added a description!")
@another.decorator()
def my_task(name: str):
    pass
