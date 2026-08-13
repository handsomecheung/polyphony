#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

TARGET_IP="${1}"
SSH_PORT="${2}"

if [[ -z "$TARGET_IP" || -z "$SSH_PORT" ]]; then
  echo "Usage: $0 <target-ip> <ssh-port>"
  exit 1
fi

# echo "box ALL=(ALL) NOPASSWD:ALL" | sudo tee /etc/sudoers.d/box
# sudo rm /etc/sudoers.d/box
ansible-playbook -i hosts node.yaml \
  -e "ansible_host=${TARGET_IP}" \
  -e "ssh_custom_port=${SSH_PORT}" \
  --ssh-common-args="-o ForwardAgent=yes"
