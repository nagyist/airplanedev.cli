import sys

import airplane

# Only raise error at runtime and not task discovery time.
if "/airplane/.airplane" in sys.path:
    raise Exception("Test")

@airplane.task()
def my_task():
    assert False
