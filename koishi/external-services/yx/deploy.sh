#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.namespace.yaml
my-k8s-deploy --file=k8s.localdev.yaml
my-k8s-deploy --file=k8s.localprod.yaml
