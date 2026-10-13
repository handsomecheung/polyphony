#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

# Deploy emulator to Kubernetes cluster
my-k8s-deploy --file=k8s.app.yaml
