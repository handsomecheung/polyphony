#!/usr/bin/env bash
set -e

mkdir -p /home/desktop/.vnc

# Set display resolution
RESOLUTION="${RESOLUTION:-1024x768}"
export DISPLAY=:0

# Create xstartup file
cat >/home/desktop/.vnc/xstartup <<'EOF'
#!/bin/sh
unset SESSION_MANAGER
unset DBUS_SESSION_BUS_ADDRESS
/usr/bin/startxfce4 &
EOF
chmod +x /home/desktop/.vnc/xstartup

# Start supervisor
exec /usr/bin/supervisord -c /etc/supervisord.conf
