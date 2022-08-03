import ast
import dataclasses
import importlib.util
import json
import os
import sys
from typing import Any, Dict, List, Literal, Optional, Union


@dataclasses.dataclass
class Param:
    slug: str
    name: str
    type: str
    description: Optional[str] = None
    default: Optional[Any] = None
    required: Optional[bool] = None
    options: Optional[List[Any]] = None
    regex: Optional[str] = None


@dataclasses.dataclass
class Def:
    slug: str
    entrypoint_func: str
    name: Optional[str] = None
    description: Optional[str] = None
    parameters: Optional[List[Param]] = None
    require_requests: Optional[bool] = None
    allow_self_approvals: Optional[bool] = None
    timeout: Optional[int] = None
    constraints: Optional[Dict[str, str]] = None
    runtime: Optional[Union[Literal[""], Literal["workflow"]]] = None

    def asdict(self) -> Dict[str, Any]:
        def to_camel(snake: str) -> str:
            first, *others = snake.split("_")
            return "".join([first.lower(), *map(str.title, others)])

        return dataclasses.asdict(
            self,
            dict_factory=lambda kv: {to_camel(k): v for (k, v) in kv if v is not None},
        )


def extract_task_configs(files: List[str]) -> List[Def]:
    configs: List[Def] = []

    for file in files:
        with open(file) as f:
            tree = ast.parse(f.read(), filename=file)

        sys.path.append(os.path.dirname(file))
        spec = importlib.util.spec_from_file_location("airplane_task_import", file)
        assert spec is not None, f"Unable to import module {file}"
        assert spec.loader is not None, f"Unable to construct loader for module {file}"
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)

        for item in tree.body:
            if isinstance(item, ast.FunctionDef) and hasattr(module, item.name):
                func = getattr(module, item.name)
                if hasattr(func, "_Task__airplane"):
                    configs.append(
                        Def(
                            slug=func.config.slug,
                            entrypoint_func=item.name,
                            name=func.config.name,
                            description=func.config.description,
                            parameters=[
                                Param(
                                    slug=param.slug,
                                    name=param.name,
                                    type=param.type,
                                    required=param.required,
                                )
                                for param in func.config.parameters
                            ],
                            require_requests=func.config.require_requests,
                            allow_self_approvals=func.config.allow_self_approvals,
                            timeout=func.config.timeout,
                            constraints=func.config.constraints,
                            runtime=func.config.runtime,
                        )
                    )

    return configs


def main() -> None:
    files = sys.argv[1:]
    task_configs = extract_task_configs(files)
    print(json.dumps([config.asdict() for config in task_configs]))


if __name__ == "__main__":
    main()
