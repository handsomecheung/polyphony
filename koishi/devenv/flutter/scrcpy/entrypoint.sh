#!/usr/bin/env bash
set -e

# Keep connecting to target ADB device if specified (retry until connected, reconnect on drop)
if [ -n "${ADB_DEVICE_ADDRESS}" ]; then
  (
    sleep 3
    while true; do
      if ! adb devices | grep -E "^${ADB_DEVICE_ADDRESS}[[:space:]]+device$" >/dev/null; then
        echo "Connecting to ADB device at ${ADB_DEVICE_ADDRESS}..."
        adb connect "${ADB_DEVICE_ADDRESS}" || true
      fi
      sleep 10
    done
  ) &
fi

exec "$@"
