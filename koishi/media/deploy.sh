#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

./openlist/deploy.sh
./rclone/deploy.sh

./bliss/deploy.sh
./plex/deploy.sh
./samba/deploy.sh

./backup/deploy.sh
