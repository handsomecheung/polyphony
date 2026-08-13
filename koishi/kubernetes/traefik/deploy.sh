#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

kubectl apply -f k8s.traefik-config.yaml
kubectl apply -f k8s.middleware.yaml

my-k8s-deploy --file=k8s.traefik-dashboard.yaml
my-k8s-deploy --file=k8s.networkpolicy.yaml

# kubectl get helmchartconfig traefik -n kube-system -o yaml
# kubectl get helmchart traefik -n kube-system
# kubectl rollout status deployment/traefik -n kube-system
