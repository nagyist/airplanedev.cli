import ast
import json
import re
import sys
import traceback
from typing import Any, Dict

from exceptions import TaskNotFound
from location import Location
from parse import Parser
from serialize import Serializer
from utils import apply_updates, get_indent


def update(
    file: str,
    slug: str,
    task_def: Dict[str, Any],
    py_version: str = "",
) -> str:
    new_slug = task_def.get("slug", "")
    assert new_slug != ""

    with open(file, "r", encoding="utf-8") as f:
        content = f.read()
        root = ast.parse(content, filename=file)

        p = Parser(slug, content)
        p.visit(root)

        if not p.decorator_loc or not p.func_name or not p.func_def_loc:
            raise TaskNotFound(slug)

        # If the function name matches the new (or old) slug, don't set it in the decorator since
        # it can be extracted from the function name.
        func_name = p.func_name
        if p.func_name in (slug, new_slug):
            func_name = new_slug
            task_def.pop("slug")

        s = Serializer(get_indent(content), py_version)

        serialized_task_def = s.serialize_task_def(task_def)
        serialized_func_def = s.serialize_func_def(
            func_name, task_def.get("parameters", None), is_async=p.is_async
        )
        serialized_imports = s.serialize_imports(p.imports)
        # Add a trailing newline if we're adding at least one import, since we'll be
        # placing these imports in an existing empty line.
        serialized_imports += "\n" if len(serialized_imports) > 0 else ""

        return apply_updates(
            content,
            [
                (p.decorator_loc, serialized_task_def),
                (p.func_def_loc, serialized_func_def),
                (get_import_loc(content, root), serialized_imports),
            ],
        )


def get_import_loc(content: str, root: ast.Module) -> Location:
    # Initialize the import_loc to the first non-comment line
    # (e.g. hashbangs/docstrings/comments).
    lineno = 0

    # If there is a leading docstring, skip it.
    docs = ast.get_docstring(root)
    if docs:
        i = content.index(docs)
        m = re.match(r'^(^.*?""")', content[i:], flags=re.DOTALL)
        if m:
            i += len(m.group(0))
            lineno = len(content[0:i].split("\n"))

    # Skip past lines that have a comment.
    lines = content.split("\n")
    for i, line in enumerate(lines, start=lineno):
        if re.match(r"^($|(?!#).*$)", line):
            lineno = i
            break

    return Location(
        Location.offset(content, lineno, 0), Location.offset(content, lineno, 0)
    )


def main() -> None:
    command = sys.argv[1]
    assert command
    file = sys.argv[2]
    assert file
    slug = sys.argv[3]
    assert slug

    if command == "can_update":
        can_update = True
        try:
            update(file, slug, {"slug": slug})
        # pylint: disable-next=broad-exception-caught
        except Exception:
            print(traceback.format_exc(), file=sys.stderr)
            can_update = False
        print(f"__airplane_output {json.dumps(can_update)}")
    elif command == "update":
        serialized_task_def = sys.argv[4]
        assert serialized_task_def
        task_def = json.loads(serialized_task_def)
        py_version = sys.argv[5]

        contents = update(file, slug, task_def, py_version=py_version)
        with open(file, "w", encoding="utf-8") as f:
            f.write(contents)
    else:
        raise ValueError(f"unknown command: {command}")
