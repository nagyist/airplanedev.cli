import airplane


@airplane.task(
    constraints={
        "a_valid_identifier": "...",
        "both'\"'\"": "'\"'\"",
        'double"': '"',
        "single'": "'",
    }
)
def my_task():
    pass
