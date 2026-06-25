#!/usr/bin/env bash
set -ex

cd "$(dirname "${BASH_SOURCE[0]}")/"
source .env

pids=$(pgrep -f "runner --server wss://${ARONDO_HOST_SERVER_PROD}/runner --name devbox-nur" || true)
if [ -n "$pids" ]; then
  echo "Stopping previous runner processes: $pids"
  kill $pids || true
  for pid in $pids; do
    while kill -0 $pid 2>/dev/null; do
      sleep 0.1
    done
  done
fi

file_bin=/tmp/arondo.prod.runner
file_log=/tmp/arondo.prod.runner.log

wget https://github.com/handsomecheung/Arondo/releases/latest/download/runner-linux-amd64 -O "${file_bin}"
chmod +x "${file_bin}"
nohup bash -c "/tmp/arondo.prod.runner --server wss://${ARONDO_HOST_SERVER_PROD}/runner --name devbox-nur" >"${file_log}" 2>&1 &
tail -f "${file_log}"
