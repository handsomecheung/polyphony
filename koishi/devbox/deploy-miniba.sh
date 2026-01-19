#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-deploy --file=k8s.app.others.yaml --var.name=devbox-miniba --var.tcp-port=37002 --var.run-on-host=miniba
