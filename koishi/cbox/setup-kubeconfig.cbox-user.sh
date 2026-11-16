#!/usr/bin/env bash
set -euo pipefail
umask 077

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
kubectl_bin=${KUBECTL_BIN:-kubectl}
output=${1:-/mnt/coder-workspaces/private-workspace/others/c/coder-workspaces/private-workspace/kube/config}

if [[ "$output" != /* ]]; then
    echo "The output path must be absolute." >&2
    exit 1
fi

command -v "$kubectl_bin" >/dev/null
command -v python3 >/dev/null
scratch=$(mktemp -d)
output_tmp=
trap 'rm -rf "$scratch"; if [[ -n "$output_tmp" ]]; then rm -f "$output_tmp"; fi' EXIT

# Copy only cluster connection settings, never the administrator's credentials.
"$kubectl_bin" config view --raw --minify --flatten -o json |
    python3 -c 'import json, sys; print(json.dumps(json.load(sys.stdin)["clusters"][0]["cluster"]))' >"$scratch/cluster.json"

"$kubectl_bin" get namespace ck-dev builder -o name
"$kubectl_bin" apply -f "$script_dir/k8s.cbox-user.yaml"
"$kubectl_bin" delete clusterrolebinding cbox-user-binding --ignore-not-found

# The token controller fills the Secret asynchronously.
ready=false
for ((attempt = 0; attempt < 30; attempt++)); do
    "$kubectl_bin" get secret cbox-user-token -n kube-system -o json >"$scratch/secret.json"
    if python3 -c 'import json, sys; sys.exit(not json.load(sys.stdin).get("data", {}).get("token"))' <"$scratch/secret.json"; then
        ready=true
        break
    fi
    sleep 1
done
if [[ "$ready" != true ]]; then
    echo "The token controller has not populated cbox-user-token. Retry after checking the controller." >&2
    exit 1
fi

mkdir -p "$(dirname "$output")"
output_tmp=$(mktemp "$(dirname "$output")/.cbox-config.XXXXXX")
python3 - "$scratch/cluster.json" "$scratch/secret.json" "$output_tmp" <<'PY'
import base64
import json
import sys

with open(sys.argv[1]) as source:
    cluster = json.load(source)
with open(sys.argv[2]) as source:
    token = base64.b64decode(json.load(source)["data"]["token"]).decode()
config = {
    "apiVersion": "v1",
    "kind": "Config",
    "clusters": [{"name": "cbox", "cluster": cluster}],
    "users": [{"name": "cbox-user", "user": {"token": token}}],
    "contexts": [
        {"name": namespace, "context": {"cluster": "cbox", "user": "cbox-user", "namespace": namespace}}
        for namespace in ("ck-dev", "builder")
    ],
    "current-context": "ck-dev",
}
with open(sys.argv[3], "w") as target:
    json.dump(config, target, indent=2)
    target.write("\n")
PY

# Check with the generated credentials, not administrator impersonation.
for namespace in ck-dev builder; do
    "$kubectl_bin" --kubeconfig="$output_tmp" auth can-i '*' '*' -n "$namespace" --quiet
done
for namespace in default others kube-system; do
    if "$kubectl_bin" --kubeconfig="$output_tmp" auth can-i get secrets -n "$namespace" --quiet; then
        echo "Unexpected access to $namespace. Check other RBAC bindings for cbox-user." >&2
        exit 1
    else
        status=$?
        [[ "$status" -eq 1 ]] || exit "$status"
    fi
done
# Clear the context namespace when reviewing a cluster-scoped resource.
if "$kubectl_bin" --kubeconfig="$output_tmp" auth can-i get nodes --all-namespaces --quiet; then
    echo "Unexpected access to nodes. Check other RBAC bindings for cbox-user." >&2
    exit 1
else
    status=$?
    [[ "$status" -eq 1 ]] || exit "$status"
fi

mv -fT "$output_tmp" "$output"
output_tmp=
echo "Created $output (mode 600). The token was not printed."
