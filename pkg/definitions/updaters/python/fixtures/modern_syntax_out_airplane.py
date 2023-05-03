import airplane


@airplane.task(description="Added a description!")
async def my_task(name: str) -> int:
    return 10
