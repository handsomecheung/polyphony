#!/usr/bin/env bash
set -e

RULE_SETS_DIR="$(dirname "$0")"

echo "Downloading sing-box rule sets to $RULE_SETS_DIR..."

echo "Downloading geosite-cn.srs..."
curl -L -o "$RULE_SETS_DIR/geosite-cn.srs" \
  "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs"

echo "Downloading geosite-private.srs..."
curl -L -o "$RULE_SETS_DIR/geosite-private.srs" \
  "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-private.srs"

echo "Downloading geoip-cn.srs..."
curl -L -o "$RULE_SETS_DIR/geoip-cn.srs" \
  "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs"

echo "All rule sets downloaded successfully!"
echo "File sizes:"
ls -lh "$RULE_SETS_DIR"/*.srs
