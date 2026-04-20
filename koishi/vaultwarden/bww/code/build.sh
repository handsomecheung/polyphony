#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
CGO_ENABLED=0 go build -o bww main.go
