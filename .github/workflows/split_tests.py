#!/usr/bin/env python3
""" split_tests.py

    Determine the golang tests to run based on the shard.
"""

import subprocess
import sys

# Packages to put in shard 0. Everything else will be in shard 1.
#
# These were chosen by hand by looking at recent test runs and picking
# out packages that were particularly slow to test.
SHARD0_PACKAGES = set([
    'github.com/airplanedev/cli/pkg/build/node/nodetest',
    'github.com/airplanedev/cli/pkg/build/python/pythontest',
    'github.com/airplanedev/cli/pkg/build/shell/shelltest',
    'github.com/airplanedev/cli/pkg/build/views/viewstest',
])


def main():
    if len(sys.argv) != 2:
        raise Exception('Usage: test_split.py [shard]')

    shard = sys.argv[1]

    if shard == '0':
        print(' '.join(sorted(list(SHARD0_PACKAGES))))
    else:
        packages_to_test = []

        for package in all_packages():
            if package not in SHARD0_PACKAGES:
                packages_to_test.append(package)

        print(' '.join(packages_to_test))


def all_packages():
    result = subprocess.check_output('go list ./...', shell=True)
    rows = result.decode('utf-8').split('\n')
    return [row.strip() for row in rows if row.strip() != '']


if __name__ == '__main__':
    main()
