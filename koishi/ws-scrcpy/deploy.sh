#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

# Deploy manifest using koishi deploy tool
my-k8s-deploy --file=k8s.app.yaml
