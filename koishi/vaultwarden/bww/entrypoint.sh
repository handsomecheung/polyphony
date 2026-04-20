#!/usr/bin/env bash
set -e

if [ -n "$BW_URL" ]; then
    bw config server "$BW_URL"
fi

if [ -n "$BW_CLIENTID" ] && [ -n "$BW_CLIENTSECRET" ]; then
    bw login --apikey
fi

if [ -n "$BW_PASSWORD" ]; then
    export BW_SESSION=$(bw unlock --passwordenv BW_PASSWORD --raw)
fi

exec /usr/local/bin/bww
