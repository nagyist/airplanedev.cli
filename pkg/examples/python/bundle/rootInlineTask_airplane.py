import airplane

@airplane.task()
def my_task(
  name: str,
):
  print("running:my_task")

@airplane.task()
def my_task2(
  name: str,
):
  print("running:my_task2")
