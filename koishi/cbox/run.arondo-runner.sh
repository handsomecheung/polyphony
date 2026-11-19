#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")/"
source ./.env

bin=/tmp/arondo.runner.bin
log=/tmp/arondo.runner.log

wget https://github.com/handsomecheung/Arondo/releases/latest/download/arondo-runner-linux-amd64 -O "${bin}"
chmod +x "${bin}"
nohup bash -c "${bin} --server wss://${ARONDO_HOST_SERVER}/runner --token ${ARONDO_RUNNER_TOKEN} --auto-update=true" >"${log}" 2>&1 &
