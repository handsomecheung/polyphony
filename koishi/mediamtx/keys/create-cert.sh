#!/usr/bin/env bash
set -e


cd "$(dirname "${BASH_SOURCE[0]}")"

key=../config/server.key
cert=../config/server.crt

rm -f "${cert}"
openssl req -config openssl.cnf -new -x509 -sha256 -key "${key}" -out "${cert}" -days 3650
