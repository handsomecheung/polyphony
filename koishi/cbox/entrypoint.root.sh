#!/usr/bin/env bash
set -e

echo "try fixing permission for .local"
chown box:box /home/box/.local || true

echo "try fixing permission for .local/share"
chown box:box /home/box/.local/share || true

echo "try fixing permission for .config"
chown box:box /home/box/.config || true

echo "set password for box user"
echo "box:${BOX_PASSWORD}" | chpasswd

echo "run box entrypoint.sh"
su - box -c "bash /home/box/entrypoint.box.sh"

echo "run sshd"
/usr/sbin/sshd

echo "sleep infinity"
sleep infinity
