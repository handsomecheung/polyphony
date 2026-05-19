#!/usr/bin/env bash
set -e

echo "Installing CloudNativePG Operator v1.29.1..."
kubectl apply --server-side -f \
    https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.29/releases/cnpg-1.29.1.yaml

echo "Waiting for CloudNativePG deployment to be ready..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=cloudnative-pg -n cnpg-system --timeout=300s

echo "Installing Barman Cloud Plugin v0.12.0..."
kubectl apply -f \
    https://github.com/cloudnative-pg/plugin-barman-cloud/releases/download/v0.12.0/manifest.yaml

echo "Waiting for Barman Cloud Plugin deployment to be ready..."
kubectl rollout status deployment -n cnpg-system barman-cloud --timeout=300s

echo "CloudNativePG Operator installed successfully."
