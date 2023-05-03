import airplane


@airplane.task(name="My task")
def my_task():
    pass


@airplane.task(name="My second task")
def my_task_2():
    pass
