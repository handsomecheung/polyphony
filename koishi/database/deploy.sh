#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.app.namespace.yaml

my-k8s-deploy --file=k8s.app.redis.prod.yaml
my-k8s-deploy --file=k8s.app.redis.dev.yaml

my-k8s-deploy --file=k8s.app.postgres.dev.yaml
my-k8s-deploy --file=k8s.app.postgres.prod.yaml

./tools/deploy.sh
