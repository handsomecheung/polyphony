#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

if [[ "$EUID" -ne 0 ]]; then
  echo "This script must be run as root."
  exit 1
fi

if [[ "$(hostname)" != "nippon" ]]; then
  echo "This script must be run on master node."
  exit 1
fi

curl -sfL https://get.k3s.io | sh -s - server \
  --cluster-init \
  --flannel-backend=none \
  --disable-network-policy \
  --cluster-cidr=10.42.0.0/16,2001:cafe:42::/56 \
  --service-cidr=10.43.0.0/16,2001:cafe:43::/112

TOKEN=$(cat /var/lib/rancher/k3s/server/node-token)
echo "TOKEN: $TOKEN"

echo "copy kubeconfig to your home directory:"
echo "sudo vim -p /etc/rancher/k3s/k3s.yaml ~/.kube/config"
