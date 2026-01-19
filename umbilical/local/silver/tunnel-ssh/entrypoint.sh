#!/usr/bin/env sh
set -e

rm -rf /root/.ssh
cp -r /tmp/ssh /root/.ssh
chown -R root:root /root/.ssh
chmod -R 600 /root/.ssh
chmod 700 /root/.ssh

autossh -M 30000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:37371:__{{infra.machine.silver:f:ip}}__:3737" "root@jserver.public.__{{infra.domains:f:u}}__"

tail -f /dev/null
