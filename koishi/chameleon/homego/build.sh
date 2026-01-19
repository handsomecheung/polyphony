#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-build-image "cloudpublic/chameleon/homego:latest" chameleon homego
