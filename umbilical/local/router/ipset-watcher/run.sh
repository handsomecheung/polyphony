#!/usr/bin/env bash
set -e

cd "$(dirname "${0}")"
mkdir -p /var/log/ipset-watcher
while true; do
    ./watch.sh >>/var/log/ipset-watcher/log.log 2>&1
    sleep 600
done
