#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

cargo run --quiet -- --config-dir configs/check
