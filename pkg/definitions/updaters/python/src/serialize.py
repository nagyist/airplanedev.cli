import dataclasses
import os
import re
import sys
from datetime import datetime
from typing import Any, Dict, List, Optional, Union

import black
import inflection

from utils import Imports, camel_to_snake, insert_after


@dataclasses.dataclass(frozen=True)
class SerializedValue:
    value: str

    def __str__(self) -> str:
        return self.value


class Serializer:
    def __init__(self, indent: str, py_version: str):
        self._indent = indent
        self._py_version = py_version
        self.expected_imports = Imports()
        # "airplane" must always be imported for @airplane.task.
        self.expected_imports.add("airplane")

    def serialize_imports(self, existing_imports: Imports) -> str:
        serialized_imports = []
        keys = list(self.expected_imports.imports.keys())
        for name in sorted(keys):
            imp = self.expected_imports.imports[name]
            if imp.direct and not existing_imports.has(name):
                serialized_imports.append(f"import {name}")
            includes = sorted(
                include
                for include in imp.includes
                if not existing_imports.has(name, include)
            )
            if len(includes) > 0:
                serialized_imports.append(f'from {name} import {", ".join(includes)}')

        out = "\n".join(serialized_imports)
        return format_code(out, self._indent)

    def _serialize(self, value: Any) -> str:
        if isinstance(value, SerializedValue):
            return value.value
        if value is None:
            return "None"
        if isinstance(value, str):
            # Escape double quotes.
            return '"' + value.replace('"', '\\"') + '"'
        if isinstance(value, (int, float, bool)):
            return str(value)
        if isinstance(value, list):
            return "[" + ",".join([self._serialize(elem) for elem in value]) + "]"
        if isinstance(value, dict):
            kvs = ",".join(
                [
                    self._serialize(key) + ": " + self._serialize(value[key])
                    for key in value
                ]
            )
            return "{" + kvs + "}"
        raise ValueError(f"unable to serialize value: {value}")

    def _serialize_env_var(self, key: str, value: Dict) -> str:
        name = self._serialize(key)
        if "config" in value:
            config = self._serialize(value["config"])
            return f"airplane.EnvVar(name={name}, config_var_name={config})"
        v = self._serialize(value["value"])
        return f"airplane.EnvVar(name={name}, value={v})"

    def _serialize_param_option(self, opt: Dict, param: Dict) -> str:
        if "label" in opt:
            label = self._serialize(opt["label"])
            value = self._serialize_param_value(opt["value"], param)
            return f"airplane.LabeledOption(label={label}, value={value})"
        return self._serialize_param_value(opt, param)

    def _serialize_parameter(self, param: Dict) -> str:
        py_type = {
            "shorttext": "str",
            "longtext": "airplane.LongText",
            "sql": "airplane.SQL",
            "boolean": "bool",
            "integer": "int",
            "float": "float",
            "upload": "airplane.File",
            "date": "date",
            "datetime": "datetime",
            "configvar": "airplane.ConfigVar",
        }.get(param["type"], None)
        if py_type is None:
            raise ValueError(f"Unknown parameter type: {param['type']}")

        if py_type == "date":
            self.expected_imports.add("datetime", "date")
        elif py_type == "datetime":
            self.expected_imports.add("datetime", "datetime")

        required = param.get("required", True)
        if not required:
            if self._is_py_version_gte("3.10"):
                py_type = f"{py_type} | None"
            else:
                self.expected_imports.add("typing", "Optional")
                py_type = f"Optional[{py_type}]"

        # Wrap the parameter as Annotated if it has fields that require airplane.ParamConfig.
        #
        # If the name can be generated from the slug, don't bother serializing the name.
        conditional_fields: List[str] = []
        slug = param.get("slug", "")
        if inflection.humanize(slug) == param.get("name", ""):
            conditional_fields.append("name")
        annotated_param = {
            k: param[k]
            for k in param
            # These fields can all be conveyed without Annotated.
            # This updater doesn't (yet?) support docstrings, so description is not included.
            if k not in ["slug", "type", "default", "required"] + conditional_fields
        }
        if annotated_param:
            if "options" in annotated_param:
                annotated_param["options"] = [
                    SerializedValue(self._serialize_param_option(opt, param))
                    for opt in annotated_param["options"]
                ]
            if self._is_py_version_gte("3.9"):
                self.expected_imports.add("typing", "Annotated")
            else:
                self.expected_imports.add("typing_extensions", "Annotated")
            kwargs = self._serialize_kwargs(annotated_param)
            py_type = f"Annotated[{py_type}, airplane.ParamConfig({kwargs})]"

        default = param.get("default", None)
        # If the parameter is optional, set the default as `None` unless a default is
        # explicitly specified.
        if default is not None or not required:
            py_type += f" = {self._serialize_param_value(default, param)}"

        return slug + ": " + py_type

    def _serialize_parameters(self, params: Optional[Dict]) -> str:
        if not params:
            return ""

        # Serialize parameters that do _not_ have defaults first. Optional parameters will get a
        # default of "None".
        out = []
        has_default = [
            param.get("default", None) is not None or not param.get("required", True)
            for param in params
        ]
        for i, param in enumerate(params):
            if not has_default[i]:
                out.append(self._serialize_parameter(param))
        for i, param in enumerate(params):
            if has_default[i]:
                out.append(self._serialize_parameter(param))

        return ", ".join(out)

    def serialize_func_def(
        self, func_name: str, params: Optional[Dict], is_async: bool
    ) -> str:
        serialized_params = self._serialize_parameters(params)
        decl = "async def" if is_async else "def"
        out = f"{decl} {func_name}({serialized_params})"

        # Temporarily attach a function body so that this function definition becomes
        # valid Python code.
        n = format_code(out + ":\n    pass", self._indent)
        # Remove the temporary suffix (note that the indenting may have changed thus
        # we use self.indent!)
        return n[0 : -(len(":\npass") + len(self._indent))]

    def _serialize_param_value(self, value: Any, param: Optional[Dict]) -> str:
        t = param.get("type", None) if param else None
        if t == "datetime" and isinstance(value, str):
            value = value[0 : value.index("Z")]
            d = datetime.strptime(
                # Handle optional milliseconds.
                value,
                "%Y-%m-%dT%H:%M:%S.%f" if "." in value else "%Y-%m-%dT%H:%M:%S",
            )
            self.expected_imports.add("datetime", "timezone")
            self.expected_imports.add("datetime", "datetime")
            components = (
                f"{d.year}, {d.month}, {d.day}, {d.hour}, {d.minute}, {d.second}"
            )
            if d.microsecond > 0:
                components += f", {d.microsecond}"
            return f"datetime({components}, tzinfo=timezone.utc)"
        if t == "date" and isinstance(value, str):
            d = datetime.fromisoformat(value)
            self.expected_imports.add("datetime", "date")
            return f"date({d.year}, {d.month}, {d.day})"
        if t == "configvar":
            if isinstance(value, str):
                return f"airplane.ConfigVar({self._serialize(value)})"
            if "config" in value and isinstance(value["config"], str):
                return f"airplane.ConfigVar({self._serialize(value['config'])})"

        return self._serialize(value)

    def _serialize_param_values(self, values: Dict, params: List[Dict]) -> str:
        items = []
        for slug, value in values.items():
            param = next((p for p in params if p.get("slug") == slug), None)
            items.append(
                [slug, SerializedValue(self._serialize_param_value(value, param))]
            )
        return self._serialize(dict(items))

    def _serialize_schedule(self, slug: str, schedule: Dict, params: List[Dict]) -> str:
        if "paramValues" in schedule:
            schedule["paramValues"] = SerializedValue(
                self._serialize_param_values(schedule["paramValues"], params)
            )
        kwargs = [
            camel_to_snake(key) + "=" + self._serialize(value)
            for key, value in schedule.items()
        ]
        return f"airplane.Schedule(slug={self._serialize(slug)}, {', '.join(kwargs)})"

    def _serialize_resource(self, slug: str, alias: Optional[str] = None) -> str:
        if alias is None or alias == slug:
            return f"airplane.Resource({self._serialize(slug)})"
        return f"airplane.Resource({self._serialize(slug)}, alias={self._serialize(alias)})"

    def _serialize_resources(self, resources: Union[List[str], Dict[str, str]]) -> str:
        # Standardize resources on a dict.
        resources = (
            {slug: slug for slug in resources}
            if isinstance(resources, list)
            else resources
        )
        return self._serialize(
            [
                SerializedValue(self._serialize_resource(slug, alias))
                for alias, slug in resources.items()
            ]
        )

    def serialize_task_def(self, task_def: Dict[str, Any]) -> str:
        # Copy the task_def so we can mutate it.
        task_def = task_def.copy()

        # Extract python-specific fields.
        if "python" in task_def:
            if "envVars" in task_def["python"]:
                # Rewrite env vars as a list instead of an object.
                env_vars = [
                    SerializedValue(self._serialize_env_var(key, value))
                    for key, value in task_def["python"]["envVars"].items()
                ]
                task_def = insert_after(task_def, "python", "envVars", env_vars)
            task_def.pop("python")

        if "resources" in task_def:
            task_def["resources"] = SerializedValue(
                self._serialize_resources(task_def["resources"])
            )

        if "schedules" in task_def:
            params = task_def.get("parameters", [])
            task_def["schedules"] = [
                SerializedValue(self._serialize_schedule(slug, s, params))
                for slug, s in task_def["schedules"].items()
            ]

        # Parameters are set via function parameters.
        if "parameters" in task_def:
            task_def.pop("parameters")

        # If the timeout is the default value, don't serialize it.
        if task_def.get("timeout", 0) == 3600:
            task_def.pop("timeout")

        # Format the generated code using black. Use a "_" as a placeholder so black doesn't
        # error on it being invalid.
        out = "_airplane.task(" + self._serialize_kwargs(task_def) + ")"
        return "@" + format_code(out, self._indent)[1:]

    def _serialize_kwargs(self, kwargs: Dict) -> str:
        out = []
        for key, value in kwargs.items():
            out.append(camel_to_snake(key) + "=" + self._serialize(value))
        return ", ".join(out)

    def _is_py_version_gte(self, version: str) -> bool:
        """Returns true if the version of Python used to execute the serialized code is
        >= version."""
        # Strip off the `3.` prefix.
        min_version = int(version[2:])

        # If the user has an airplane.yaml with a `python.version` field, that value
        # will be passed in as `py_version`. This represents the version of Python
        # that will execute this task once deployed. However, the serialized code also
        # needs to work with the _current_ version of Python otherwise this code won't
        # execute in Studio. Therefore, compare against the minimum of these versions.
        py_version = int(self._py_version[2:]) if self._py_version else None
        if os.getenv("TESTING_ONLY_IGNORE_CURRENT_PYTHON_VERSION") == "true":
            if py_version is None:
                raise ValueError("expected an airplane.yaml python version")
        else:
            py_version = (
                min(py_version, sys.version_info.minor)
                if py_version
                else sys.version_info.minor
            )

        return py_version >= min_version


def format_code(code: str, indent: str) -> str:
    """
    Formats a chunk of valid Python code. This requires black, which we package
    inline (via PEX) so that users don't need it installed.
    """
    formatted = black.format_str(code, mode=black.Mode())
    # Black will append a newline. Remove it.
    formatted = formatted[0:-1]

    lines = [
        # Replace the indentation in the formatted code (which will always be 4 spaces)
        # with the indentation in the user's file.
        re.sub("^(    )+", lambda m: (indent * int((len(m.group(0)) / 4))), line)
        for line in formatted.split("\n")
    ]
    return "\n".join(lines)
