#!/usr/bin/env bash
set -ex

cd "$(dirname "${BASH_SOURCE[0]}")/"
source ./.env

file_bin=/tmp/arondo.runner.bin
file_log=/tmp/arondo.runner.nur.log

pids=$(pgrep -f "runner --server wss://${ARONDO_HOST_SERVER_PROD}/runner --token ${ARONDO_RUNNER_TOKEN_NUR}" || true)
if [ -n "$pids" ]; then
  echo "Stopping previous runner processes: $pids"
  kill $pids || true
  for pid in $pids; do
    while kill -0 $pid 2>/dev/null; do
      sleep 0.1
    done
  done
fi

wget https://github.com/handsomecheung/Arondo/releases/latest/download/arondo-runner-linux-amd64 -O "${file_bin}"
chmod +x "${file_bin}"
nohup bash -c "${file_bin} --server wss://${ARONDO_HOST_SERVER_PROD}/runner --token ${ARONDO_RUNNER_TOKEN_NUR}" >"${file_log}" 2>&1 &
# tail -f "${file_log}"
