#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.middleware.yaml

./aria2/deploy.sh
./ariang/deploy.sh
./flaresolverr/deploy.sh
./jackett/deploy.sh
./qbittorrent/deploy.sh
./qbittorrent-vuetorrent/deploy.sh
