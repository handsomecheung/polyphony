#!/usr/bin/env sh
set -e

rm -rf /root/.ssh
cp -r /tmp/ssh /root/.ssh
chown -R root:root /root/.ssh
chmod -R 600 /root/.ssh
chmod 700 /root/.ssh

autossh -M 30000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:37371:{{ tacet://*/infra.machine.silver/ip }}:3737" "root@jserver.public.{{ tacet://*/infra.domains/u }}"

tail -f /dev/null
