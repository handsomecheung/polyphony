#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

helm uninstall coder coder-v2/coder --namespace my-coder
