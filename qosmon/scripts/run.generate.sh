#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

rm -rf ../configs/check/auto-generated
python3.12 generate.py --config ../configs/generate/config.yaml
