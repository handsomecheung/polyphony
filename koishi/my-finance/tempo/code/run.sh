#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

source .env

python3.12 main.py
