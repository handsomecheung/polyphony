#!/usr/bin/env sh
set -e

chmod -R 600 /root/.ssh

# 1. from jserver to router
autossh -M 21000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:80:__{{infra.machine.r3b:f:ip}}__:80" -R "eth0:443:__{{infra.machine.r3b:f:ip}}__:443" -R "eth0:1083:__{{infra.machine.r3b:f:ip}}__:1083" "root@jserver.public.__{{infra.domains:f:u}}__"

# 2. from jserver to silver
autossh -M 22000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:3737:__{{infra.machine.silver:f:ip}}__:3737" "root@jserver.public.__{{infra.domains:f:u}}__"

# 3. from jserver to silver-cbox
autossh -M 23000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:3738:__{{infra.machine.silver:f:ip}}__:3738" "root@jserver.public.__{{infra.domains:f:u}}__"

# 4. from jserver to silver-gbox
autossh -M 24000 -q -f -N -o "ServerAliveInterval 10" -o "ServerAliveCountMax 3" -R "eth0:3739:__{{infra.machine.silver:f:ip}}__:3739" "root@jserver.public.__{{infra.domains:f:u}}__"

tail -f /dev/null
