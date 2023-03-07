import dataclasses
from typing import Any, Callable, Dict, List, Optional


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
    restrict_callers: Optional[List[str]]
    timeout: Optional[int]
    constraints: Optional[Dict[str, str]]
    resources: Optional[List[Any]]
    schedules: Optional[List[Any]]
    parameters: Optional[List[ParamDef]]
    env_vars: Optional[Any]
