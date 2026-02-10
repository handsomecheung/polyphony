#!/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

CGO_ENABLED=0 GOOS=linux go build -o runner ./cmd/server/main.go
