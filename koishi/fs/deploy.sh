#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.app.codeserver.yaml

my-k8s-deploy --file=k8s.app.downserver.yaml
my-k8s-deploy --file=k8s.app.downserver.private.yaml
my-k8s-deploy --file=k8s.app.downserver.cksns.yaml

my-k8s-deploy --file=k8s.app.upserver.yaml
my-k8s-deploy --file=k8s.app.upserver.cksns.yaml
