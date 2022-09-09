# This file includes a shim that will execute your task code.

try:
    import airplane
except ModuleNotFoundError:
    pass
import importlib.util as util
import json
import os
import sys
import traceback

def run(args):
    sys.path.append("{{.TaskRoot}}")

    if len(args) != 2:
        err_msg = "usage: python ./shim.py <args>"
        print(err_msg, file=sys.stderr)
        airplane.set_output(err_msg, "error")
        sys.exit(1)

    os.chdir("{{.TaskRoot}}")

    {{if .EntrypointFunc}}
    module_name = "mod.{{.EntrypointFunc}}"
    {{else}}
    module_name = "mod.main"
    {{end}}
    spec = util.spec_from_file_location(module_name, "{{.Entrypoint}}")
    mod = util.module_from_spec(spec)
    spec.loader.exec_module(mod)

    try:
        {{if .EntrypointFunc}}
        ret = mod.{{.EntrypointFunc}}.base_func(json.loads(args[1]))
        {{else}}
        ret = mod.main(json.loads(args[1]))
        {{end}}
        if ret is not None:
            try:
                airplane.set_output(ret)
            except NameError:
                # airplanesdk is not installed - gracefully print to stdout instead.
                # This makes it easier to use the shim in a dev environment. We ensure airplanesdk
                # is installed in production images.
                sys.stdout.flush()
                print("The airplanesdk package must be installed to set return values as task output.", file=sys.stderr)
                print("Printing return values to stdout instead.", file=sys.stderr)
                sys.stderr.flush()
                print(json.dumps(ret, indent=2))
    except Exception as e:
        print(traceback.format_exc(), file=sys.stderr)
        airplane.set_output(str(e), "error")
        sys.exit(1)

if __name__ == "__main__":
    run(sys.argv)
