#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

service=$1

if [ "${2}" = "--build" ]; then
    docker compose build "${service}"
fi

docker compose stop "${service}"
docker compose rm -f "${service}"

docker compose up -d "${service}"
