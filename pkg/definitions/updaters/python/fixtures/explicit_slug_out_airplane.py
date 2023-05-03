import airplane


@airplane.task(slug="my_task_2", description="Added a description!")
def run(name: str):
    pass
