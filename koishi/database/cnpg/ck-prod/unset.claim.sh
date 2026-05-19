#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

for name in postgres-1 postgres-2; do
    pv=$(kubectl get pv -o jsonpath="{.items[?(@.spec.claimRef.name=='$name')].metadata.name}")
    if [ -n "$pv" ]; then
        echo "Patching PV: $pv (formerly bound to $name)"
        kubectl patch pv "$pv" -p '{"spec":{"claimRef":null}}'
    else
        echo "PV for $name not found."
    fi
done
