#!/usr/bin/env python3
""" copy_cache_images.py

    This script ensures that each of the images referred to in versions.json
    is in our public cache in both stage and prod.
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
                digest = image['digest']

                logging.info(f'Copying {img_type}:{tag}')
                subprocess.check_output([
                    'docker',
                    'buildx',
                    'imagetools',
                    'create',
                    '-t', f'us-central1-docker.pkg.dev/airplane-stage/public-cache/{img_type}:{tag}',
                    '-t', f'us-central1-docker.pkg.dev/airplane-prod/public-cache/{img_type}:{tag}',
                    f'docker.io/{img_type}@{digest}',
                ])

    logging.info('Done')


if __name__ == '__main__':
    main()
