#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

./aria2/deploy.sh
./ariang/deploy.sh
./flaresolverr/deploy.sh
./jackett/deploy.sh
./qbittorrent/deploy.sh
./qbittorrent-vuetorrent/deploy.sh
