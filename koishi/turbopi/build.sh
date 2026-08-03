#!/usr/bin/env bash
# Build script for turbopi webapp using Kaniko in-cluster builder
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-build-image "cloudpublic/default/turbopi:latest" default turbopi
