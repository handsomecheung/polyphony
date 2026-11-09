#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

kubectl create namespace cloudflare --dry-run=client -o yaml | kubectl apply -f -
kubectl create configmap reqanalytics-code \
  --namespace cloudflare \
  --from-file=fetcher.py=code/fetcher.py \
  --dry-run=client -o yaml | kubectl apply -f -
my-k8s-deploy --file=k8s.app.yaml
