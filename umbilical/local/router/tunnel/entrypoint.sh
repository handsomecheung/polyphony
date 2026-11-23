#!/usr/bin/env sh
set -e

chmod -R 600 /root/.ssh

# 1. from jserver to router
autossh -M 21000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:80:{{ tacet://*/infra.machine.r3b/ip }}:80" -R "eth0:443:{{ tacet://*/infra.machine.r3b/ip }}:443" -R "eth0:1083:{{ tacet://*/infra.machine.r3b/ip }}:1083" "root@jserver.public.{{ tacet://*/infra.domains/u }}"

# 2. from jserver to silver
autossh -M 22000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:3737:{{ tacet://*/infra.machine.silver/ip }}:3737" "root@jserver.public.{{ tacet://*/infra.domains/u }}"

# 3. from jserver to silver-cbox
autossh -M 23000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:3738:{{ tacet://*/infra.machine.silver/ip }}:3738" "root@jserver.public.{{ tacet://*/infra.domains/u }}"

# 4. from jserver to silver-gbox
autossh -M 24000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:3739:{{ tacet://*/infra.machine.silver/ip }}:3739" "root@jserver.public.{{ tacet://*/infra.domains/u }}"

tail -f /dev/null
