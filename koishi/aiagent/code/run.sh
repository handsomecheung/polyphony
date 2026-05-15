#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
source ../.env

export AI_MODEL_TIMEOUT_SECONDS=45
export AI_AGENT_HEARTBEAT_SECONDS=5
export AI_OUTPUT_DIR="/mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/aiagent/output"
export SKILLS_ROOT="/mnt/coder-workspaces/private-workspace/repos/local/notebook/obsidian/Default/AI/Skills"

# python3.12 main.py --serve

# python3.12 main.py "What is CPU of machine miniba?"

# python3.12 main.py "Query CPU information of machine miniba by SKILL prag"

python3.12 main.py "Please create a 15-slide presentation on generative AI implementation strategies."
