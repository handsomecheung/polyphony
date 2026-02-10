#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

kubectl delete -f k8s.app.yaml || true
my-k8s-deploy --file=k8s.app.yaml
echo "wait seconds and run command: kubectl -n default logs job/macbind -f"
sleep 10
kubectl -n default logs job/macbind -f
