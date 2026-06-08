#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-build-image "cloudpublic/ck/fountain-core/scanner:latest" ck-prod fountain-scanner
