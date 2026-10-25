#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

current=$(pwd)
cd ../../qosmon/scripts/

rm -rf "${current}/configs/check/auto-generated"
python3.12 generate.py --config "${current}/configs/generate/config.yaml" --reject-internal-ingress
