#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"
source .env

echo "=================================================="
echo " TurboPi Camera Control Webapp Starting..."
echo "=================================================="
echo ""
echo "This webapp allows you to:"
echo "1. Stream the TurboPi camera (proxied internally)"
echo "2. Control the camera servo positions via JSON-RPC"
echo ""
echo "To view the dashboard from your local machine:"
echo "  a. Run this on your local machine:"
echo "     ssh -L 3000:localhost:3000 devbox"
echo "  b. Open your browser and navigate to:"
echo "     http://localhost:3000"
echo "=================================================="
echo ""

# Run the node app
npm start
