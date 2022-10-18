import dataclasses
import datetime
import inspect
import typing
from typing import (
    Any,
    Callable,
    Dict,
    Generic,
    List,
    Literal,
    Optional,
    Sequence,
    TypeVar,
    Union,
)


# Inlined partial airplanesdk library so the parser works properly.
@dataclasses.dataclass
class ParamDef:
    arg_name: str
    slug: str
    name: str
    type: Any
    description: Optional[str]
    default: Optional[Any]
    required: Optional[bool]
    options: Optional[Any]
    regex: Optional[str]


@dataclasses.dataclass
class TaskDef:
    func: Callable[..., Any]
    slug: str
    name: str
    runtime: Any
    entrypoint_func: str
    description: Optional[str]
    require_requests: Optional[bool]
    allow_self_approvals: Optional[bool]
    timeout: Optional[int]
    constraints: Optional[Dict[str, str]]
    resources: Optional[List[Any]]
    schedules: Optional[List[Any]]
    parameters: Optional[List[ParamDef]]
    env_vars: Optional[Any]


def collatz(num: int) -> int:
    if num % 2 == 0:
        return num // 2
    return num * 3 + 1


collatz.__airplane = TaskDef(
    func=collatz,
    slug="collatz",
    name="Collatz Conjecture Step",
    entrypoint_func="collatz",
    runtime="",
    description=None,
    require_requests=False,
    allow_self_approvals=False,
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
