#!/usr/bin/env bash
set -e

export CGO_ENABLED=0
go build -ldflags="-s -w" -o runner main.go
