#!/usr/bin/env bash
set -e

declare -A macs=([nur]="${MAC_NUR}" [miniba]="${MAC_MINIBA}" [nippon]="${MAC_NIPPON}")

node=$1

echo "try to wake up node: ${node} ..."

if [ -z "${node}" ]; then
	echo "node is empty!"
	exit 1
fi

if [ -z "${KOISHI_IPV4_PREFIX}" ]; then
	echo "KOISHI_IPV4_PREFIX is empty!"
	exit 1
fi

nodes=" $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}') "

if [[ "${nodes}" != *" ${node} "* ]]; then
	echo "node ${node} is not in kubernetes cluster: ${nodes}"
	exit 2
fi

mac=${macs[$node]}
interface=$(ifconfig | grep -B1 "${KOISHI_IPV4_PREFIX}" | head -n 1 | awk -F: '{print $1}')

echo "wake up ${node}(${mac}) with interface ${interface} ..."
etherwake -i "${interface}" "${mac}"

echo "uncordon ${node} ..."
kubectl uncordon "${node}"

echo "done"
