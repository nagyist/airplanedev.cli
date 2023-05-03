import json
import sys
import traceback

if __name__ == "__main__":
    try:
        # We don't support 3.7 because the ast package does not include
        # `end_lineno / end_col_offset` values on nodes.
        # As of the time of writing this code, 3.7 hits EOL in 2 months.
        v = sys.version_info
        v_str = f"{v.major}.{v.minor}.{v.micro}"
        print(f"Using Python {v_str}")
        if sys.version_info < (3, 8):
            raise NotImplementedError(
                f"Editing inline Python tasks requires Python 3.8 (got {v_str})"
            )
        from update import main

        main()
    # pylint: disable-next=broad-exception-caught
    except Exception as e:
        print(traceback.format_exc(), file=sys.stderr)
        print(f"__airplane_error {json.dumps(str(e))}", file=sys.stderr)
        sys.stdout.flush()
        sys.stderr.flush()
        sys.exit(1)
