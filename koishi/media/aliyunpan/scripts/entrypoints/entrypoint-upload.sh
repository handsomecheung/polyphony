#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

bash mount.sh /data/plain/hero /backup-encrypted/hero --reverse
bash mount.sh /data/plain/eupload /backup-encrypted/eupload --reverse
bash mount.sh /data/plain/others-eupload /backup-encrypted/others-eupload --reverse --allow-empty

../upload-latest.py
