#!/usr/bin/env sh
set -e

chmod -R 600 /root/.ssh

autossh -M 50000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:3701:__{{infra.machine.r3b:f:ip}}__:3701" "root@jserver.public.__{{infra.domains:f:u}}__"

tail -f /dev/null
