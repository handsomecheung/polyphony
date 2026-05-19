#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

kubectl delete -f k8s.app.import.toberemove.yaml
