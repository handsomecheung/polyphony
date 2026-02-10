#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.0/cert-manager.yaml

my-k8s-deploy --file=token.cloudflare-api-token.yaml
my-k8s-deploy --file=cluster-issuer-dns-cloudflare.yaml
