#!/usr/bin/env bash
set -e

echo "try fixing permission for .local"
chown box:box /home/box/.local || true

echo "try fixing permission for .local/share"
chown box:box /home/box/.local/share || true

echo "try fixing permission for .config"
chown box:box /home/box/.config || true

echo "try fixing permission for .cursor-server"
chown box:box /home/box/.cursor-server || true

echo "set password for box user"
echo "box:${BOX_PASSWORD}" | chpasswd

echo "run box entrypoint.sh"
su - box -c "bash /home/box/entrypoint.box.sh"

echo "move kubernetes config to root dir"
ln -s /mnt/coder-workspaces/private-workspace/workspace/encrypted-configs/.kube /root/.kube

echo "run sshd"
echo "AuthorizedKeysFile .ssh/authorized_keys.$(hostname)" >>/etc/ssh/sshd_config
/usr/sbin/sshd

echo "sleep infinity"
sleep infinity
