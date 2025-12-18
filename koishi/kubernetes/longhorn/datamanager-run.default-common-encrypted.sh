#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-deploy --file=datamanager.default-common-encrypted.yaml
