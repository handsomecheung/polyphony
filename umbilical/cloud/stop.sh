#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

service=$1

docker compose stop "${service}"
docker compose rm -f "${service}"
