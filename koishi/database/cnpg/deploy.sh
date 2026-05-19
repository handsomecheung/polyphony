#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

./ck-dev/deploy.sh
./ck-prod/deploy.sh
