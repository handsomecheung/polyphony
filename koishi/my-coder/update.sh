#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-deploy --meta=pullsecret-cloudprivate --namespace=my-coder
my-k8s-deploy --file=k8s.middleware.yaml

dir=$(mktemp -d -t k8s-deploy-coder-XXXXXX)
cp values.yaml "${dir}"
my-secret render "${dir}"
helm repo update
helm upgrade coder coder-v2/coder --namespace my-coder -f "${dir}/values.yaml"
rm -rf "${dir}"
