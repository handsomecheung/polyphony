#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

FILE="ips.priority.txt"
rm -f "${FILE}"

dig +short "jserver.public.__{{infra.domains:f:u}}__" >>${FILE}
dig +short "home.__{{infra.domains:f:x}}__" >>${FILE}
