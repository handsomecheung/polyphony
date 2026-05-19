#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.app.namespace.yaml

my-k8s-deploy --file=k8s.app.redis.ck-prod.yaml
my-k8s-deploy --file=k8s.app.redis.ck-dev.yaml

./local/deploy.sh

./cnpg/deploy.sh

./tools/deploy.sh
