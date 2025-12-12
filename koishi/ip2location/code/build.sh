#!/bin/sh
set -e


export CGO_ENABLED=0
go build -o runner
