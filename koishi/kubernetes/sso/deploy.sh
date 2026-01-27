#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.namespace.yaml
my-k8s-deploy --file=k8s.domainx.yaml
my-k8s-deploy --file=k8s.domaint.yaml
my-k8s-deploy --file=k8s.domainy.yaml
my-k8s-deploy --file=k8s.domainp.yaml
my-k8s-deploy --file=k8s.domainc.yaml
