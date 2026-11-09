#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.namespace.yaml

bash fetcher/deploy.sh
bash reviewer/deploy.sh
