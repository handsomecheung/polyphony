#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

service=$1
sudo docker compose stop "${service}"
sudo docker compose rm -f "${service}"
