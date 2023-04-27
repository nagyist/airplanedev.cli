from definitions import ParamDef, TaskDef
from task_a_airplane import task_a


def task_b(num: int) -> int:
    task_a(num)
    return num


task_b.__airplane = TaskDef(
    func=task_b,
    slug="task_b",
    name="Task B",
    entrypoint_func="task_b",
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
)
