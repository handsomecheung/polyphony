#!/bin/bash
set -e

service=$1

cd "$(dirname "${BASH_SOURCE[0]}")"

sudo docker compose stop "${service}"
sudo docker compose rm -f "${service}"
