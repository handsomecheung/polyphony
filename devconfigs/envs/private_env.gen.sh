#!/bin/bash
set -e

cat <<EOF >private_env.sh
# for gemini cli
export GOOGLE_CLOUD_PROJECT="$(tacet-wrapper read "tacet://*/koishi.deploy.cloudprivate_registry_id/password")"

# for mb64
export MBKEY="$(tacet-wrapper read "tacet://*/chameleon.mb64-key/password")"

export OLLAMA_HOST=ollama:80
EOF
