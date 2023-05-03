import airplane


@airplane.task()
async def my_task() -> int:
    return 10
