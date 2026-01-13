#!/usr/bin/env bash
set -e

/opt/code-server/bin/code-server --auth none --bind-addr 0.0.0.0:8000 /workspace
