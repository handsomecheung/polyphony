#!/bin/bash
set -eo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

kubectl apply -n argo -f k8s.namespace.yaml
kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/download/v3.5.4/install.yaml

echo "Waiting for Argo Workflows to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/workflow-controller -n argo
kubectl wait --for=condition=available --timeout=300s deployment/argo-server -n argo

echo "Setup complete!"

echo "  # Access Argo UI (if needed)"
echo "  kubectl port-forward svc/argo-server -n argo 2746:2746"
echo "  # Then visit: https://localhost:2746"
