#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
my-k8s-build-image "cloudpublic/default/fs-upserver:latest" default fs-upserver
kubectl -n default rollout restart deployment fs-upserver-downcc
