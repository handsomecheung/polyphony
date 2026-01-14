#!/bin/bash
set -e

cd /mnt/coder-workspaces/public-workspace/thirdpart-repos/certbot-dns-aliyun
my-k8s-build-image "cloudpublic/default/dns-aliyun:latest"
