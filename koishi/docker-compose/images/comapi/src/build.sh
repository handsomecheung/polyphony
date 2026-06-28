#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")/.."

rustup target add x86_64-unknown-linux-musl
cargo build --release --target=x86_64-unknown-linux-musl
