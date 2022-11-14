import airplane

@airplane.task()
def my_task3(
  name: str,
):
  print("running:my_task3")
