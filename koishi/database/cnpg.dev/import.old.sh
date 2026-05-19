#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.app.import.toberemove.yaml
my-k8s-deploy --file=k8s.app.import.cluster.yaml
