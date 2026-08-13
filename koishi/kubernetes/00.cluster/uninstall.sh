#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

if [[ "$EUID" -ne 0 ]]; then
  echo "This script must be run as root."
  exit 1
fi

/usr/local/bin/k3s-uninstall.sh

/usr/local/bin/k3s-agent-uninstall.sh

ip link show
