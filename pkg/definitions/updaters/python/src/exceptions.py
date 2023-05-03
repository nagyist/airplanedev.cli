class TaskNotFound(Exception):
    """Raised when a task could not be found in the provided file."""

    def __init__(self, slug: str) -> None:
        super().__init__(f'Could not find task with slug "{slug}"')
