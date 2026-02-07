#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-build-image "cloudpublic/ck/fountain/scanner:latest" ck-fountain-prod scanner
