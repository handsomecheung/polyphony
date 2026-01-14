#!/bin/bash
set -e

service=$1

cd "$(dirname "${BASH_SOURCE[0]}")"

bash restart.sh "${service}" --build
