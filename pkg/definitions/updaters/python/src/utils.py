import dataclasses
import re
from collections import defaultdict
from typing import Any, Dict, List, Optional, Tuple

from location import Location


@dataclasses.dataclass()
class Import:
    direct: bool = False
    includes: List[str] = dataclasses.field(default_factory=lambda: [])


class Imports:
    def __init__(self) -> None:
        self.imports: Dict[str, Import] = defaultdict(Import)

    def add(self, module: str, include: Optional[str] = None) -> None:
        imp = self.imports[module]
        self.imports[module].direct = imp.direct or include is None
        if include and include not in imp.includes:
            self.imports[module].includes.append(include)

    def has(self, module: str, include: Optional[str] = None) -> bool:
        if module not in self.imports:
            return False
        imp = self.imports[module]
        if include is None:
            return imp.direct
        return include in imp.includes

    def __str__(self) -> str:
        return str(self.imports)


def get_indent(content: str) -> str:
    matches = re.findall(r"^[ \t]+", content, flags=re.RegexFlag.MULTILINE)
    return "    " if len(matches) == 0 else matches[0]


def insert_after(d: Dict, key: str, new_key: str, new_value: Any) -> Dict:
    idx = list(d.keys()).index(key)
    items = list(d.items())
    items.insert(idx, (new_key, new_value))
    return dict(items)


def camel_to_snake(s: str) -> str:
    return re.sub(r"(?<!^)(?=[A-Z])", "_", s).lower()


def apply_updates(content: str, updates: List[Tuple[Location, str]]) -> str:
    updates.sort(key=lambda a: a[0].start)
    for i, update in enumerate(updates):
        if i > 0:
            # Updates should not overlap (e.g. the range of chars for updating imports
            # should never overlap with the range of chars for updating the decorator).
            # If this happens, there's likely a bug and erroring here is better than
            # mucking up the user's file.
            assert update[0].start >= updates[i - 1][0].end

    # Apply updates in reverse order so that the indexes of prior updates are not impacted.
    for loc, value in reversed(updates):
        content = content[0 : loc.start] + value + content[loc.end :]

    return content
