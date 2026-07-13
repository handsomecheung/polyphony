#!/bin/bash
set -e

cat <<EOF >private_env.sh
# for gemini cli
export GOOGLE_CLOUD_PROJECT="$(bwww get-password koishi.deploy.cloudprivate_registry_id 2>/dev/null)"

# for mb64
export MBKEY="$(bwww get-password chameleon.mb64-key 2>/dev/null)"

export OLLAMA_HOST=ollama:80
EOF
