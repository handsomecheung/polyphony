#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-deploy --file=app-others.yaml --var.name=devbox-nippon --var.tcp-port=37003 --var.run-on-host=nippon
