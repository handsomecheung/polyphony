#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

kubectl delete -f k8s.app.yaml --ignore-not-found=true
my-k8s-deploy --file=k8s.app.yaml

echo "Waiting for pod to be ready..."
until kubectl -n default get pod -l job-name=macbind 2>/dev/null | grep -q macbind; do
    sleep 1
done

kubectl -n default wait --for=condition=Ready pod -l job-name=macbind --timeout=120s

echo "Pod is ready, streaming logs:"
kubectl -n default logs job/macbind -f
