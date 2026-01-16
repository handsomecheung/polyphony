#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

bash mount.sh /mnt/webdav/openlist/aliyunpan/workspace/e /mnt/decrypted/aliyunpan/workspace
bash mount.sh /mnt/webdav/openlist/aliyunpan/archived-encrypted /mnt/decrypted/aliyunpan/archived

echo "direcotrys mounted. sleep ..."
while true; do
    sleep 5
done
