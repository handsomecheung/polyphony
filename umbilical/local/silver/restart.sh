#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

service=$1
if [ "${2}" = "--build" ]; then
    sudo docker compose build "${service}"
fi

sudo docker compose stop "${service}"
sudo docker compose rm -f "${service}"
sudo docker compose up -d "${service}"
