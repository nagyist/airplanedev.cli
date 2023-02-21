#!/bin/bash

entrypoints=$1
indexhtml=$2
maintsx=$3

for entrypoint in $entrypoints; do
    # Make a directory for each entrypoint. The directory is the entrypoint without the extension.
    mkdir -p "${entrypoint}"

    # Create an index.html file for each entrypoint.
    cp $indexhtml "${entrypoint}"

    # Create a main.tsx file for each entrypoint. Replace {{.Entrypoint}} with /path/to/entrypoint
    sed -e "s|{{.Entrypoint}}|\/$entrypoint|" $maintsx > "${entrypoint}/main.tsx"
done
