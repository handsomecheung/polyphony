#!/usr/bin/env bash
set -e

# Connect to target ADB device if specified
if [ -n "${ADB_DEVICE_ADDRESS}" ]; then
  (
    sleep 3
    echo "Connecting to ADB device at ${ADB_DEVICE_ADDRESS}..."
    adb connect "${ADB_DEVICE_ADDRESS}" || true
    adb devices
  ) &
fi

exec "$@"
