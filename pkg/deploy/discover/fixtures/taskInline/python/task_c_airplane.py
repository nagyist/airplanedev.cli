import functools

from definitions import ParamDef, TaskDef
from task_a_airplane import task_a
from task_b_airplane import task_b


def task_c(num: int) -> int:
    task_a(num)
    task_b(num)
    return num


task_c.__airplane = TaskDef(
    func=task_c,
    slug="task_c",
    name="Task C",
    entrypoint_func="task_c",
    runtime="",
    description=None,
    require_requests=False,
    allow_self_approvals=False,
    restrict_callers=None,
    timeout=3600,
    concurrency_key=None,
    concurrency_limit=None,
    constraints=None,
    resources=None,
    schedules=None,
    env_vars=None,
    parameters=[
        ParamDef(
            arg_name="num",
            slug="num",
            name="Num",
            type="integer",
            description=None,
            default=None,
            required=True,
            options=None,
            regex=None,
        )
    ],
    sdk_version=None,
)


def wrap_task_d():
    def decorator(func):
        config = TaskDef(
            func=func,
            slug="task_d",
            name="Task D",
            entrypoint_func="task_d",
            runtime="",
            description=None,
            require_requests=False,
            allow_self_approvals=False,
            restrict_callers=None,
            timeout=3600,
            concurrency_key=None,
            concurrency_limit=None,
            constraints=None,
            resources=None,
            schedules=None,
            env_vars=None,
            parameters=[
                ParamDef(
                    arg_name="num",
                    slug="num",
                    name="Num",
                    type="integer",
                    description=None,
                    default=None,
                    required=True,
                    options=None,
                    regex=None,
                )
            ],
            sdk_version=None,
        )

        @functools.wraps(func)
        def wrapped(*args, **kwargs):
            return "hello"

        wrapped.__airplane = config
        return wrapped

    return decorator


def _task_d(num: int) -> int:
    return num


# Define a task via a variable assignment
task_d = wrap_task_d()(_task_d)
