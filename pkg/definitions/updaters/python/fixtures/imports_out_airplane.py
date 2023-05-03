"""
This module has a docstring!
"""
from datetime import datetime
import airplane

from something import *
import somethingelse as thing

if THING:
    import bar


@airplane.task(description="Added a description!")
def my_task(datetime: datetime):
    pass
