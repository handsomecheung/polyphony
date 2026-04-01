#!/bin/sh
set -e

cd "$(dirname "$0")"
CGO_ENABLED=0 go build -o main ./cmd/server/main.go
