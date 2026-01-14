#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
rm -rf target
my-k8s-build-image "cloudpublic/compose/comapi:latest"
