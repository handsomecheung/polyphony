#!/usr/bin/env bash
set -e

mkdir -p /mnt/user-data-slight/gphotosync/hh/camera
mkdir -p /mnt/user-data-slight/gphotosync/hh/screenshot
mkdir -p /mnt/user-data-slight/gphotosync/hh/others
chown -R 1000:1000 /mnt/user-data-slight/gphotosync/hh || true

mkdir -p /mnt/user-data-slight/gphotosync/cc/sync
chown -R 1001:1001 /mnt/user-data-slight/gphotosync/cc || true

mkdir -p /mnt/user-data-slight/gphotosync/001/sync
chown -R 1002:1002 /mnt/user-data-slight/gphotosync/001 || true

cp -va /tmp/gphotosync-sshkeys-system/. /etc/ssh/
chown -R root:root /etc/ssh/*
chmod go-r /etc/ssh/*

USER_HH="photo-sync-__{{infra.common-users:f:hh}}__"
USER_CC="photo-sync-__{{infra.common-users:f:cc}}__"

cp -r /tmp/gphotosync-sshkeys-user-hh /home/${USER_HH}/.ssh
chown -R ${USER_HH}:${USER_HH} /home/${USER_HH}
chmod 700 /home/${USER_HH}/.ssh
chmod 600 /home/${USER_HH}/.ssh/*

cp -r /tmp/gphotosync-sshkeys-user-cc /home/${USER_CC}/.ssh
chown -R ${USER_CC}:${USER_CC} /home/${USER_CC}
chmod 700 /home/${USER_CC}/.ssh
chmod 600 /home/${USER_CC}/.ssh/*

cp -r /tmp/gphotosync-sshkeys-user-001 /home/photo-sync-001/.ssh
chown -R photo-sync-001:photo-sync-001 /home/photo-sync-001
chmod 700 /home/photo-sync-001/.ssh
chmod 600 /home/photo-sync-001/.ssh/*

mkdir -p /run/sshd
/usr/sbin/sshd -D
