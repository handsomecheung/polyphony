#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

TARGET_BIN_DIR="${HOME}/bin"
mkdir -p "${TARGET_BIN_DIR}"

echo "Building release binaries for tacet..."
cd "${PROJECT_DIR}"
cargo build --release --bin tacet --bin tacetd

echo "Installing binaries to ${TARGET_BIN_DIR}..."
install -m 755 "${PROJECT_DIR}/target/release/tacet" "${TARGET_BIN_DIR}/tacet"
install -m 755 "${PROJECT_DIR}/target/release/tacetd" "${TARGET_BIN_DIR}/tacetd"

# Terminate running daemon so next invocation auto-spawns the updated binary
if pkill -x tacetd 2>/dev/null; then
    echo "Terminated existing tacetd daemon (will auto-spawn on next command)."
fi

echo "Build and installation complete: ${TARGET_BIN_DIR}/tacet and ${TARGET_BIN_DIR}/tacetd"
