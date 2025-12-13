#!/bin/bash
set -eo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-build-image cloudpublic/base/ubuntu:24.04
