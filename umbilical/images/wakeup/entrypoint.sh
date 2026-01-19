#!/usr/bin/env bash
set -e

echo "run gotty ..."
# gotty --permit-arguments tmux new -A -s main /entrypoint.gotty.sh
gotty --permit-arguments /entrypoint.gotty.sh


# echo "sleep infinity ..."
# sleep infinity