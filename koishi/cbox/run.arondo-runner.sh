#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")/"
source ./.env

bin=/tmp/arondo.runner.bin
log=/tmp/arondo.runner.log

wget https://github.com/handsomecheung/Arondo/releases/latest/download/arondo-runner-linux-amd64 -O "${bin}"
chmod +x "${bin}"
export ARONDO_RUNNER_BINARY="${bin}"
export ARONDO_RUNNER_SERVER="wss://${ARONDO_HOST_SERVER}/runner"
export ARONDO_RUNNER_TOKEN
nohup /bin/zsh -ic 'export SHELL=/bin/zsh; exec "$ARONDO_RUNNER_BINARY" --server "$ARONDO_RUNNER_SERVER" --auto-update=true' >"${log}" 2>&1 &
