#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

gcp_sa_file=/tmp/qosmon-gcs-upload.json

if [[ ! -f "${gcp_sa_file}" ]]; then
    bwww get-attachment koishi-qosmon qosmon-gcs-upload.json >"${gcp_sa_file}"
fi

cargo run --quiet -- --config-dir configs/check --concurrency 50 --only-failures --format plain
