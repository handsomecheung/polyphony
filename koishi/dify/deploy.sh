#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

# Create namespace
my-k8s-deploy --file=k8s.app.namespace.yaml

# Apply storage configuration (PV/PVC)
my-k8s-deploy --file=k8s.storage.yaml

# Deploy local PostgreSQL
my-k8s-deploy --file=k8s.postgres.yaml

# Prepare values
dir=$(mktemp -d -t k8s-deploy-dify-XXXXXX)
cp values.yaml "${dir}"
bwww render-file "${dir}"

# Add repo
helm repo add dify https://langgenius.github.io/dify-helm
helm repo update

# Deploy
# Note: Using 'dify' as the release name and 'dify' as the namespace
helm upgrade --install dify dify/dify --namespace dify -f "${dir}/values.yaml"

rm -rf "${dir}"
