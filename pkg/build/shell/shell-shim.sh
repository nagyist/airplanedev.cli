#!/bin/bash

# Params are passed in as param_slug_1=value1, param_slug_2=value2
# Export as environment variables, PARAM_SLUG_1=value1, PARAM_SLUG_2=value2
sep="="
for param in "${@:2}"; do
    # Split into slug and value by separator. Taken from https://unix.stackexchange.com/a/53323.
    case $param in
        (*"$sep"*)
            param_slug=${param%%"$sep"*}
            param_value=${param#*"$sep"}
            ;;
        (*)
            param_slug=$param
            param_value=
            ;;
    esac
    # Convert to uppercase
    var_name="$(echo "PARAM_${param_slug}" | tr '[:lower:]' '[:upper:]')"
    # Export env var
    export "${var_name}"="${param_value}"
done

exec "$1"
