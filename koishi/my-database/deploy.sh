#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=app.namespace.yaml

my-k8s-deploy --file=app.redis.yaml

my-k8s-deploy --file=app.postgres.dev.yaml
my-k8s-deploy --file=app.postgres.prod.yaml

my-k8s-deploy --file=app.dbgate.yaml
my-k8s-deploy --file=app.dbgate.cc.yaml
