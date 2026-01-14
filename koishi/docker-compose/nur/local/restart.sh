#!/bin/bash
set -e

service=$1

cd "$(dirname "${BASH_SOURCE[0]}")"

if [ "${2}" = "--build" ]; then
    docker-compose build ${service}
fi

sudo docker compose stop ${service}
sudo docker compose rm -f ${service}

sudo docker compose up -d ${service}
