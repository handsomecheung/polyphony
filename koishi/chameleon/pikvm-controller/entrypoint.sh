#!/bin/sh
set -e

mkdir -p /root/.ssh
cp /tmp/ssh-config/config /root/.ssh/config
cp /tmp/ssh-keys/* /root/.ssh/

chmod -R 600 /root/.ssh/

python -u server.py &
python -u controller.py
