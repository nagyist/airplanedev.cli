import ast
from typing import List, Optional, Union

from location import Location
from utils import Imports


# Note: The Python AST module does not include comments, but we could extract them using
# either of the following libraries (which are recommended in the official Python AST docs):
# - https://asttokens.readthedocs.io/en/latest/user-guide.html
# - https://libcst.readthedocs.io/en/latest/
#
# The following links are very useful in understanding the Python AST:
# - https://greentreesnakes.readthedocs.io/en/latest/tofrom.html
# - https://python-ast-explorer.com/
#
# pylint: disable-next=too-many-instance-attributes
class Parser(ast.NodeVisitor):
    def __init__(self, slug: str, content: str):
        self._slug = slug
        self._content = content
        # Track if the parser is currently traversing within a class.
        self._current_class: Optional[ast.ClassDef] = None

        self.imports = Imports()
        self.decorator_loc: Optional[Location] = None
        self.func_def_loc: Optional[Location] = None
        self.func_name: Optional[str] = None
        self.is_async = False

    def visit_ClassDef(self, node: ast.ClassDef) -> None:
        self._current_class = node
        super().generic_visit(node)
        self._current_class = None

    def visit_Import(self, node: ast.Import) -> None:
        self.imports.add(node.names[0].name)

    def visit_ImportFrom(self, node: ast.ImportFrom) -> None:
        assert node.module is not None
        self.imports.add(node.module, node.names[0].name)

    def visit_FunctionDef(self, node: ast.FunctionDef) -> None:
        self._generic_visit_func_def(node)

    def visit_AsyncFunctionDef(self, node: ast.AsyncFunctionDef) -> None:
        self._generic_visit_func_def(node)

    def _generic_visit_func_def(
        self, func: Union[ast.FunctionDef, ast.AsyncFunctionDef]
    ) -> None:
        # Ignore function definitions within classes.
        if self._current_class is not None:
            return

        self.func_name = func.name

        # Compute the location of the function definition. This includes the entire function from
        # `def...` up to and including the closing parenthesis for the arg list.
        func_loc = Location.from_node(func, self._content)
        # We have to compute the arg length. Note that we can't compute the end based on the
        # location of the return type (it may not be present) nor the first line of the body
        # (there may be comments before that line, which are not included in the AST).
        decl = "def (" if isinstance(func, ast.FunctionDef) else "async def("
        args_start = func_loc.start + len(decl) + len(func.name)
        args_end = args_start
        # For docs on the various AST args, see:
        # https://docs.python.org/3/library/ast.html#ast.arguments
        args: List[Union[ast.expr, ast.arg]] = []
        defaults: List[ast.expr] = []
        defaults.extend(func.args.defaults)
        defaults.extend([d for d in func.args.kw_defaults if d is not None])
        args.extend(func.args.args)
        args.extend(func.args.posonlyargs)
        args.extend(func.args.kwonlyargs)
        args.extend(func.args.defaults)
        args.extend(defaults)
        args.extend([a for a in [func.args.vararg, func.args.kwarg] if a is not None])
        for arg in args:
            loc = Location.from_node(arg, self._content)
            args_start = min(args_start, loc.start)
            args_end = max(args_end, loc.end)

        # There may be a trailing comma after the last arg. We need to include that in args_end.
        for i in range(args_end, len(self._content)):
            if self._content[i] == ")":
                args_end = i
                break
        self.func_def_loc = Location(func_loc.start, args_end + len(")"))

        # Check if this function has the @airplane.task decorator with a matching slug.
        dec = self._get_airplane_decorator(func)
        if not dec or not self._slug_matches(func, dec):
            # This task does not match. Continue parsing...
            self.generic_visit(func)
            return

        if self._has_computed_fields(dec, defaults):
            raise ValueError("Tasks that use computed fields must be updated manually.")

        self.decorator_loc = Location.from_node(dec, self._content)
        # The dec node is a Call (airplane.task(...)) so it does not include the leading
        # @ for the decorator. Include this in decorator_loc:
        self.decorator_loc.start -= 1
        self.is_async = isinstance(func, ast.AsyncFunctionDef)

    def _get_airplane_decorator(
        self, node: Union[ast.FunctionDef, ast.AsyncFunctionDef]
    ) -> Optional[ast.Call]:
        for dec in node.decorator_list:
            if (
                not isinstance(dec, ast.Call)
                or not isinstance(dec.func, ast.Attribute)
                or not isinstance(dec.func.value, ast.Name)
            ):
                continue
            if dec.func.value.id == "airplane" and dec.func.attr == "task":
                return dec
        return None

    def _slug_matches(
        self,
        func: Union[ast.FunctionDef, ast.AsyncFunctionDef],
        dec: ast.Call,
    ) -> bool:
        for k in dec.keywords:
            if k.arg == "slug":
                return isinstance(k.value, ast.Constant) and k.value.value == self._slug
        # If this decorator doesn't specify a slug, default to the function name.
        return func.name == self._slug

    def _has_computed_fields(self, call: ast.Call, defaults: List[ast.expr]) -> bool:
        """Returns true if the ast nodes contain only literal values."""
        p = ComputedFieldsParser()
        for k in call.keywords:
            p.visit(k.value)
        for d in defaults:
            p.visit(d)
        return p.has_computed


class ComputedFieldsParser(ast.NodeVisitor):
    def __init__(self) -> None:
        self.has_computed = False

    def visit_Constant(self, node: ast.Constant) -> None:
        pass

    def visit_List(self, node: ast.List) -> None:
        pass

    def visit_Dict(self, node: ast.Dict) -> None:
        pass

    def visit_JoinedStr(self, node: ast.JoinedStr) -> None:
        for v in node.values:
            if not isinstance(v, ast.Constant):
                self.has_computed = True

    def generic_visit(self, node: ast.AST) -> None:
        self.has_computed = True
        print("Found computed node", node)
