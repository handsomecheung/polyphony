#!/usr/bin/env bash
set -e

declare -A macs=([silver]="${MAC_SILVER}")

node=$1

echo "try to wake up node: ${node} ..."

if [ -z "${node}" ]; then
	echo "node is empty!"
	exit 1
fi

if [ -z "${UMBILICAL_IPV4_PREFIX}" ]; then
	echo "UMBILICAL_IPV4_PREFIX is empty!"
	exit 1
fi

mac=${macs[$node]}
if [ -z "${mac}" ]; then
	echo "invalid node name!"
	exit 1
fi

interface=$(ifconfig | grep -B1 "${UMBILICAL_IPV4_PREFIX}" | head -n 1 | awk -F: '{print $1}')

echo "wake up ${node}(${mac}) with interface ${interface} ..."
etherwake -i "${interface}" "${mac}"

echo "done"
