import airplane
from submod.thing import do_thing

@airplane.task(
    slug="import_task",
    name="Import Task",
)
def import_task():
    print(do_thing())
