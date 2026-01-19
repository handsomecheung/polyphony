#!/usr/bin/env bash
set -e

echo "Configure specific IPs to bypass TUN and connect directly without entering sing-box"

BYPASS_MARK=0x200 # Mark for traffic bypassing TUN
IPSET_NAME="priority"

echo "Setting up TUN bypass rules..."
ipset create ${IPSET_NAME} hash:net 2>/dev/null || true

echo ""
echo "Current bypass IPs:"
ipset list ${IPSET_NAME} | grep -E "^[0-9]" || echo "(none)"

# Clean up old iptables rules if exist
iptables -t mangle -D OUTPUT -m set --match-set ${IPSET_NAME} dst -j MARK --set-mark ${BYPASS_MARK} 2>/dev/null || true
iptables -t mangle -D PREROUTING -m set --match-set ${IPSET_NAME} dst -j MARK --set-mark ${BYPASS_MARK} 2>/dev/null || true

# Add iptables mangle rules (mark packets)
# OUTPUT: Traffic originated from router itself
iptables -t mangle -A OUTPUT -m set --match-set ${IPSET_NAME} dst -j MARK --set-mark ${BYPASS_MARK}
# PREROUTING: Forwarded traffic (e.g., from LAN devices)
iptables -t mangle -A PREROUTING -m set --match-set ${IPSET_NAME} dst -j MARK --set-mark ${BYPASS_MARK}

echo "✓ iptables mangle rules added"

# Clean up old policy routing rules if exist
ip rule del fwmark ${BYPASS_MARK} table main 2>/dev/null || true

# Add policy routing rule (priority 8000, before sing-box rules)
# Marked traffic uses main routing table directly, not table 2022 (TUN)
ip rule add fwmark ${BYPASS_MARK} table main priority 8000

echo "✓ Policy routing rule added"

echo ""
echo "=== Current configuration ==="
echo "IP rule for bypass (priority 8000):"
ip rule show | grep 8000 || echo "(not found)"

echo ""
echo "iptables mangle rules:"
iptables -t mangle -L OUTPUT -n -v | grep -A2 "Chain OUTPUT" | tail -3
iptables -t mangle -L PREROUTING -n -v | grep -A2 "Chain PREROUTING" | tail -3

echo ""
echo "=== Setup complete ==="
echo "To add IPs to bypass list:"
echo "  ipset add ${IPSET_NAME} <IP_ADDRESS>"
echo "Example:"
echo "  ipset add ${IPSET_NAME} 8.8.8.8"
