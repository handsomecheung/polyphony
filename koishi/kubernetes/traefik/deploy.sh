#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-deploy --file=k8s.traefik-dashboard.yaml
my-k8s-deploy --file=k8s.networkpolicy.yaml
