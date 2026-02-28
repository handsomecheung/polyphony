#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.gateway.yaml
my-k8s-deploy --file=k8s.node.yaml
