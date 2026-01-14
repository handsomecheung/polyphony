#!/usr/bin/env bash
set -e

echo "run gotty ..."
gotty --permit-arguments -w /entrypoint.gotty.sh
