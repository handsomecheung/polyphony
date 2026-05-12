#!/bin/bash
set -e

cd $(dirname "${BASH_SOURCE[0]}")
my-k8s-deploy --file=app.yaml
my-k8s-deploy --file=k8s.app.upserver-downcc.yaml
