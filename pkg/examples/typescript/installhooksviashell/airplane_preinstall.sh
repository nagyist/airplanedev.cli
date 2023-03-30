#!/bin/bash
set -euo pipefail

echo "hello from preinstall" > preinstall.txt

apt-get update -y \
    && apt-get upgrade -y \
    && apt-get install -y rolldice=1.16-1+b3
