#!/usr/bin/env bash
set -e


session="$1"
if [ -z "${session}" ]; then
    session="main"
fi

tmux new -A -s "${session}" bash