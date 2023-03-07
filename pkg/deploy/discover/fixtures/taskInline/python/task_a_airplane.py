from definitions import ParamDef, TaskDef


def task_a(num: int) -> int:
    return num


task_a.__airplane = TaskDef(
    func=task_a,
    slug="task_a",
    name="Task A",
    entrypoint_func="task_a",
    runtime="",
    description=None,
    require_requests=False,
    allow_self_approvals=False,
    restrict_callers=None,
    timeout=3600,
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
