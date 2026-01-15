#!/bin/bash
set -e

cd /mnt/coder-workspaces/public-workspace/thirdpart-repos
if [ ! -d "wireproxy" ]; then
    git clone https://github.com/pufferffish/wireproxy.git
fi

cd wireproxy
my-k8s-build-image "cloudpublic/default/wireproxy:latest" default wireproxy
