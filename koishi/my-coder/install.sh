#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-deploy --meta=pullsecret-cloudprivate --namespace=my-coder

dir=$(mktemp -d -t k8s-deploy-coder-XXXXXX)
cp values.yaml "${dir}"
my-secret render "${dir}"

kubectl create namespace my-coder
helm repo add coder-v2 https://helm.coder.com/v2
helm repo update

helm install coder coder-v2/coder --namespace my-coder --values "${dir}/values.yaml"
rm -rf "${dir}"
