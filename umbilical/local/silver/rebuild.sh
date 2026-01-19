#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

service=$1

bash restart.sh "${service}" --build
