#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

job=letsencrypt-refresh-umbilical

kubectl delete -f app.yaml 2>/dev/null || true
my-k8s-deploy --file=app.yaml

echo "=== Waiting for pod to be ready ==="
kubectl -n default wait --for=condition=Ready pod -l job-name=${job} --timeout=30s 2>/dev/null || true

echo "=== Init container (generate) logs ==="
kubectl -n default logs job/${job} -c generate -f 2>/dev/null || true

echo "=== Main container (copy) logs ==="
kubectl -n default logs job/${job} -c copy -f
