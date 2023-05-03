import airplane


@airplane.task(name="My task")
def my_task():
    pass


@airplane.task(name="My task (v2)")
def my_task_two():
    pass
