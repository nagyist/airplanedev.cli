import dataclasses
from dataclasses import dataclass
from typing import Any, Callable, Dict, List, Literal, Optional, TypeVar, Union


# Inlined partial airplanesdk library so the parser works properly.
@dataclass
class ParamConfig:
    slug: str
    name: str
    type: str
    description: Optional[str] = None
    default: Optional[Any] = None
    required: Optional[bool] = None
    options: Optional[List[Any]] = None
    regex: Optional[str] = None


@dataclass
class TaskConfig:
    slug: str
    name: Optional[str] = None
    description: Optional[str] = None
    parameters: Optional[List[ParamConfig]] = None
    require_requests: Optional[bool] = None
    allow_self_approvals: Optional[bool] = None
    timeout: Optional[int] = None
    constraints: Optional[Dict[str, str]] = None
    runtime: Optional[Union[Literal[""], Literal["workflow"]]] = None


TOutput = TypeVar('TOutput')
UserFunc = Callable[[Dict[str, Any]], TOutput]


@dataclass
class Task:
    __airplane: Literal[True] = dataclasses.field(default=True, init=False, repr=False)
    config: TaskConfig
    base_func: UserFunc

    def __call__(self, params: Dict[str, Any]) -> TOutput:
        return self.base_func(params)


def task(config: TaskConfig) -> Callable[[UserFunc], UserFunc]:
    def decorate(func: UserFunc) -> UserFunc:
        return Task(config, func)
    return decorate


@task(
    config=TaskConfig(
        slug="collatz",
        name="Collatz Conjecture Step",
        parameters=[ParamConfig(slug="num", name="Num", type="integer")],
    )
)
def collatz(params: Dict[str, Any]) -> int:
    num: int = params["num"]
    if num % 2 == 0:
        return num//2
    else:
        return num*3+1
