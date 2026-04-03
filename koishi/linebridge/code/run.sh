#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
bash build.sh

source .env

./main --config .config.yaml
