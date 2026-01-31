#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/v1.11.0/deploy/longhorn.yaml
my-k8s-deploy --file=app.ingress.yaml
my-k8s-deploy --file=app.volumes.yaml
my-k8s-deploy --file=app.recurring.yaml
