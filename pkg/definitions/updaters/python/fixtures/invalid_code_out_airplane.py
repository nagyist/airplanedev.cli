# This file includes a few intentional "bugs" to ensure the update gracefully
# handles them.
import airplane

# Bug: `airplane` was not imported

# Bug: unknown function reference
ipsum()


@airplane.task(description="Added a description!")
def my_task():
    # Bug: unknown variable reference(foo).
    print(foo)
