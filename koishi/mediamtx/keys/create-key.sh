#!/usr/bin/env bash
set -e


cd "$(dirname "${BASH_SOURCE[0]}")"

file=../config/server.key
rm -f ${file}
openssl genrsa -out ${file} 2048
