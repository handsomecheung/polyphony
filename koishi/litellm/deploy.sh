#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-deploy --file=k8s.configmap.yaml
kubectl -n default rollout restart deployment litellm
kubectl -n default rollout status deployment litellm --timeout=300s

my-k8s-deploy --file=k8s.app.yaml
