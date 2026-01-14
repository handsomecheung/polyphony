#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
kubectl create namespace my-coder
my-k8s-deploy --meta=pullsecret-cloudprivate --namespace=my-coder

dir=$(mktemp -d -t k8s-deploy-coder-XXXXXX)
cp values.yaml "${dir}"
my-secret render "${dir}"

helm repo add coder-v2 https://helm.coder.com/v2
helm repo update

helm install coder coder-v2/coder --namespace my-coder --values "${dir}/values.yaml"
rm -rf "${dir}"
