#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

FILE="ips.priority.txt"
rm -f "${FILE}"

dig +short "jserver.public.{{ tacet://*/infra.domains/u }}" >>${FILE}
dig +short "home.{{ tacet://*/infra.domains/x }}" >>${FILE}
