#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-build-image "cloudpublic/devenv/flutter/ws-scrcpy:latest" devenv-flutter scrcpy
