#!/usr/bin/env bash
set -e

VNC_SERVER=${VNC_SERVER:-"localhost:5900"}
WEBSOCKIFY_PORT=${WEBSOCKIFY_PORT:-4567}
DISPLAY_WIDTH=${DISPLAY_WIDTH:-1280}
DISPLAY_HEIGHT=${DISPLAY_HEIGHT:-720}

echo "Starting noVNC..."
echo "VNC server: $VNC_SERVER"
echo "Websockify port: $WEBSOCKIFY_PORT"
echo "Display size: ${DISPLAY_WIDTH}x${DISPLAY_HEIGHT}"

if [ -d /usr/share/novnc/config ]; then
    cp -r /usr/share/novnc/config/* /opt/novnc/ || true
fi

cd /opt/novnc
echo "run websockify + noVNC ..."
exec websockify --web=/opt/novnc "${WEBSOCKIFY_PORT}" "${VNC_SERVER}"
