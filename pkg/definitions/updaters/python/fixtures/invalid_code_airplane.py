# This file includes a few intentional "bugs" to ensure the update gracefully
# handles them.

# Bug: `airplane` was not imported

# Bug: unknown function reference
ipsum()


@airplane.task()
def my_task():
    # Bug: unknown variable reference(foo).
    print(foo)
