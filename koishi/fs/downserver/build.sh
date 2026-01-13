#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-build-image "cloudpublic/default/fs-downserver:latest" default fs-downserver
