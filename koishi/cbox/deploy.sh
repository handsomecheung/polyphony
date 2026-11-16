#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

bash setup-kubeconfig.cbox-user.sh

my-k8s-deploy --file=k8s.namespace.yaml
my-k8s-deploy --file=k8s.cbox-user.yaml
my-k8s-deploy --file=k8s.app.yaml
