import ast
import dataclasses

from typing_extensions import Self


@dataclasses.dataclass()
class Location:
    """Represents a range within a given string of content. The indexes are 0-based in
    the form of [start, end)."""

    start: int
    end: int

    @classmethod
    def offset(cls, content: str, lineno: int, col_offset: int) -> int:
        """Generates the 0-based offset into content given a 0-based line number
        and column offset."""
        lines = content.split("\n")
        offset = 0
        for i in range(lineno):
            # Add one to account for the newline at the end of the line.
            offset += len(lines[i]) + 1
        return offset + col_offset

    @classmethod
    def from_node(cls, node: ast.AST, content: str) -> Self:
        """Generates a Location from an AST node."""
        assert node.end_lineno
        assert node.end_col_offset

        # Note: lineno/col_offset are 1-based indexes. Subtract 1 to account for that.
        lineno = node.lineno - 1
        end_lineno = node.end_lineno - 1
        col_offset = node.col_offset - 1
        end_col_offset = node.end_col_offset - 1

        # The col_offset and end_col_offset values are based on utf-8 byte length, e.g.
        # >>> len('你'.encode('utf-8'))
        # 3
        # >>> len('你')
        # 1
        # Standardize on string length.
        def to_string_len(lineno: int, col_offset: int) -> int:
            lines = content.split("\n")
            s = lines[lineno].encode("utf-8")[0 : col_offset + 1]
            return len(s.decode("utf-8"))

        col_offset = to_string_len(lineno, col_offset)
        end_col_offset = to_string_len(end_lineno, end_col_offset)

        return cls(
            cls.offset(content, lineno, col_offset),
            cls.offset(content, end_lineno, end_col_offset),
        )
