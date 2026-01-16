#!/usr/bin/env sh
set -e

items="1:webdav-openlist:/mnt/webdav/openlist 2:webdav-pikpak:/mnt/webdav/pikpak-readonly 3:pikpak:/mnt/remote/pikpak 4:pikpak-encrypted-backup:/mnt/decrypted/pikpak/backup 5:gdrive:/mnt/remote/gdrive 6:gdrive-encrypted-backup:/mnt/decrypted/gdrive/backup"
for item in ${items}; do
    index=$(echo "${item}" | cut -d ":" -f 1)
    remote=$(echo "${item}" | cut -d ":" -f 2)
    dir=$(echo "${item}" | cut -d ":" -f 3)
    echo "mount ${index} ${remote} in ${dir} ..."
    mkdir -p "${dir}" || true
    umount "${dir}" || true
    rclone mount --allow-non-empty --allow-other --vfs-cache-mode=writes --cache-dir="/tmp/rclone-${index}" "${remote}:" "${dir}" &
done

echo "done. sleep ..."
while true; do
    sleep 5
done
