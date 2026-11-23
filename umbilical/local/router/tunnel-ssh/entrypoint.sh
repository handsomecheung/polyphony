#!/usr/bin/env sh
set -e

chmod -R 600 /root/.ssh

autossh -M 50000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:3701:{{ tacet://*/infra.machine.r3b/ip }}:3701" "root@jserver.public.{{ tacet://*/infra.domains/u }}"

tail -f /dev/null
