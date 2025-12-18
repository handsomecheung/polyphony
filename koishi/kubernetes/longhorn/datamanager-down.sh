#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

kubectl delete -f datamanager.chameleon-data.yaml || true
kubectl delete -f datamanager.default-common-encrypted.yaml || true
kubectl delete -f datamanager.vaultwarden-encrypted.yaml || true
