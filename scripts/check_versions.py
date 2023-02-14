#!/usr/bin/env python3
""" check_versions.py

    Check that tags and digests match up in versions.json.

    Note that the SHAs are not 100% stable due to security updates in the
    upstream images, but they should match for any new images that you're adding.
"""

import json
import logging
import logging.config
import os
import subprocess

logging.basicConfig(
    format='%(asctime)s [%(levelname)s]: %(message)s',
    level=logging.INFO,
)

VERSIONS_PATH = os.path.join(
    os.path.dirname(
        os.path.realpath(__file__),
    ),
    '../pkg/build/versions.json',
)


def main():
    with open(VERSIONS_PATH) as versions_file:
        versions_json = json.load(versions_file)

        for img_type, versions in versions_json.items():
            for version, image in versions.items():
                tag = image['tag']

                uri = f'docker.io/{img_type}:{tag}'
                file_digest = image['digest']
                registry_digest = get_digest(uri)

                if file_digest != registry_digest:
                    logging.warning(
                        f'Mismatched digest for {uri}: expected {registry_digest}, got {file_digest}',
                    )
                else:
                    logging.info(f'Digest for {uri} ok')

    logging.info('Done!')


def get_digest(uri):
    """Get the digest for a multi-platform image."""
    result = subprocess.check_output(
        ['docker', 'buildx', 'imagetools', 'inspect', uri],
    ).decode('utf-8')

    for row in result.split('\n'):
        if row.startswith('Digest:'):
            digest = row.split(' ')[-1]
            return digest

    return None


if __name__ == '__main__':
    main()
