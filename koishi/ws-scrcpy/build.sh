#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

# Build and push container image using Kaniko
my-k8s-build-image "cloudpublic/default/ws-scrcpy:latest" default ws-scrcpy
