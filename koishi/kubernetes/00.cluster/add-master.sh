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

# sudo cat /var/lib/rancher/k3s/server/node-token
TOKEN=

curl -sfL https://get.k3s.io | K3S_TOKEN="${TOKEN}" sh -s - server \
  --server "https://${SERVER_IP}:6443" \
  --flannel-backend=none \
  --disable-network-policy \
  --cluster-cidr=10.42.0.0/16,2001:cafe:42::/56 \
  --service-cidr=10.43.0.0/16,2001:cafe:43::/112
