#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
go test ./internal/queue -v -p 10
