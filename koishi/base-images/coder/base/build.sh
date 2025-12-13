#!/bin/bash
set -eo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-build-image "cloudpublic/coder/workspace-base:ubuntu2204"
