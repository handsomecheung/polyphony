#!/bin/bash
set -e

cat <<EOF >private_env.sh
# for gemini cli
export GOOGLE_CLOUD_PROJECT="$(tacet-wrapper read "tacet://org/Default/DevOps/Koishi/Base/infra-infos/cloudprivate-registry-id")"

# for mb64
export MBKEY="$(tacet-wrapper read "tacet://org/Default/DevOps/Koishi/CK/Chameleon/mb64/key")"

export OLLAMA_HOST=ollama:80
EOF
