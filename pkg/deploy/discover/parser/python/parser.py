import dataclasses
import importlib.util
import inspect
import json
import os
import sys
from typing import Any, Dict, List, Literal, Optional, Union


@dataclasses.dataclass
class Option:
    label: str
    value: Any


@dataclasses.dataclass
class Param:
    slug: str
    name: str
    type: str
    description: Optional[str]
    default: Optional[Any]
    required: bool
    options: Union[List[Option], List[str], None]
    regex: Optional[str]


@dataclasses.dataclass
class Schedule:
    name: Optional[str]
    description: Optional[str]
    cron: str
    paramValues: Dict[str, Any]


@dataclasses.dataclass
class EnvVar:
    value: Optional[str]
    config: Optional[str]


@dataclasses.dataclass
class PythonDef:
    envVars: Optional[Dict[str, EnvVar]]
    entrypoint: str


@dataclasses.dataclass
class Def:
    entrypointFunc: str

    name: str
    slug: str
    description: Optional[str]
    parameters: List[Param]
    resources: Optional[Dict[str, str]]

    constraints: Optional[Dict[str, str]]
    python: PythonDef
    requireRequests: bool
    allowSelfApprovals: bool
    restrictCallers: Optional[List[Literal["task", "view"]]]
    timeout: int
    runtime: Literal["standard", "workflow"]
    concurrencyKey: str
    concurrencyLimit: int

    schedules: Dict[str, Schedule]


def as_def(obj: Any) -> Union[Any, Dict[str, Any]]:
    if dataclasses.is_dataclass(obj):
        return dataclasses.asdict(
            obj,
            dict_factory=lambda kv: {k: as_def(v) for (k, v) in kv if v is not None},
        )
    return obj


def extract_task_configs(files: List[str]) -> List[Def]:
    defs: List[Def] = []

    for file in files:
        sys.path.append(os.path.dirname(file))
        spec = importlib.util.spec_from_file_location("airplane_task_import", file)
        assert spec is not None, f"Unable to import module {file}"
        assert spec.loader is not None, f"Unable to construct loader for module {file}"
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)

        for name, obj in vars(module).items():
            if callable(obj) and hasattr(obj, "__airplane"):
                conf = obj.__airplane
                # Only select tasks that were defined in the file, not imported.
                if inspect.getabsfile(conf.func) != os.path.normcase(os.path.abspath(file)):
                    continue
                defs.append(
                    Def(
                        entrypointFunc=name,
                        name=conf.name,
                        slug=conf.slug,
                        description=conf.description,
                        parameters=[
                            Param(
                                slug=param.slug,
                                name=param.name,
                                type=param.type,
                                description=param.description,
                                default=param.default,
                                required=param.required,
                                options=[
                                    Option(label=o.label, value=o.value)
                                    if hasattr(o, "label")
                                    else o
                                    for o in param.options or []
                                ],
                                regex=param.regex,
                            )
                            for param in conf.parameters
                        ],
                        resources={
                            r.alias or r.slug: r.slug for r in conf.resources or []
                        },
                        constraints=conf.constraints,
                        requireRequests=conf.require_requests,
                        allowSelfApprovals=conf.allow_self_approvals,
                        restrictCallers=conf.restrict_callers if hasattr(conf, "restrict_callers") else None,
                        timeout=conf.timeout,
                        runtime=conf.runtime,
                        concurrencyKey=conf.concurrency_key if hasattr(conf, "concurrency_key") else "",
                        concurrencyLimit=conf.concurrency_limit if hasattr(conf, "concurrency_limit") else 1,
                        schedules={
                            s.slug: Schedule(
                                name=s.name,
                                description=s.description,
                                cron=s.cron,
                                paramValues=s.param_values,
                            )
                            for s in conf.schedules or []
                        },
                        python=PythonDef(
                            envVars={
                                e.name: EnvVar(
                                    value=e.value,
                                    config=e.config_var_name,
                                )
                                for e in conf.env_vars or []
                            },
                            entrypoint=file,
                        ),
                    )
                )

    return defs


def main() -> None:
    # Add the task root to the sys path. This will have no effect for local dev but will allow
    # discovered modules to properly import their dependencies during deployments.
    sys.path.append("/airplane/")
    files = sys.argv[1:]
    task_configs = extract_task_configs(files)
    print(
        "EXTRACTED_ENTITY_CONFIGS:"
        + json.dumps([as_def(config) for config in task_configs])
    )


if __name__ == "__main__":
    main()
