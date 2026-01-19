#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
../../scripts/my-build-image.sh --name "cloudpublic/umbilical/vouch:latest-arm" --platform linux/arm64/v8 "$@"
