#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
kubectl apply -f app.install.yaml
my-k8s-deploy --file=app.ingress.yaml
my-k8s-deploy --file=app.volumes.yaml
my-k8s-deploy --file=app.recurring.yaml
