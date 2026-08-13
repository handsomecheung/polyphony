#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

if [[ "$EUID" -ne 0 ]]; then
  echo "This script must be run as root."
  exit 1
fi

SERVER_IP="${1}"
if [[ -z "$SERVER_IP" ]]; then
  echo "Usage: $0 <server-ip>"
  exit 1
fi

TOKEN=
curl -sfL https://get.k3s.io | K3S_TOKEN="${TOKEN}" sh -s - agent --server "https://${SERVER_IP}:6443"
