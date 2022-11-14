# This file includes a shim that will execute your task code.

try:
    import airplane
except ModuleNotFoundError:
    pass
import importlib.util as util
import inspect
import json
import os
import sys
import traceback

def run(args):
    sys.path.append("{{.TaskRoot}}")

    if len(args) != 4:
        err_msg = "usage: python ./shim.py <entrypoint> <entrypointFunc> <args>"
        print(err_msg, file=sys.stderr)
        airplane.set_output(err_msg, "error")
        sys.exit(1)

    os.chdir("{{.TaskRoot}}")

    entrypoint = args[1]
    entrypointFunc = args[2]
    params = args[3]

    if entrypointFunc:
        module_name = "mod."+entrypointFunc
    else:
        module_name = "mod.main"
    spec = util.spec_from_file_location(module_name, entrypoint)
    mod = util.module_from_spec(spec)
    spec.loader.exec_module(mod)

    try:
        arg_dict = json.loads(params)
        if entrypointFunc:
            func = getattr(mod, entrypointFunc)
            ret = func.__airplane.run(arg_dict)
        else:
            main_example = """
    ```
    def main(params):
    print(params)
    ```
    """
            if not hasattr(mod, "main"):
                raise Exception(f"""Task is missing a `main` function. Add a main function like so and re-deploy:
    {main_example}""")
            num_params = len(inspect.signature(mod.main).parameters)
            # If the task doesn't have any parameters
            if not arg_dict:
                if num_params == 0:
                    ret = mod.main()
                elif num_params == 1:
                    ret = mod.main(arg_dict)
                else:
                    raise Exception(f"""`main` function must have at most 1 parameter, found {num_params}. Update the main function like so and re-deploy:
    {main_example}""")
            else:
                if num_params == 1:
                    ret = mod.main(arg_dict)
                else:
                    raise Exception(f"""`main` function must have exactly 1 parameter, found {num_params}. Update the main function like so and re-deploy:
    {main_example}""")
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
