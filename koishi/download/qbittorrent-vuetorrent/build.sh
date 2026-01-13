#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-build-image "cloudpublic/default/qbittorrent-vuetorrent:latest" default qbittorrent-vuetorrent
