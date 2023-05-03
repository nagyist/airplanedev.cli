import airplane


@decorator()
@airplane.task()
@another.decorator()
def my_task():
    pass
