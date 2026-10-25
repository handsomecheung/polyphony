#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

source run.env.sh
bash run.common.sh

export GOOGLE_APPLICATION_CREDENTIALS="${GOOGLE_APPLICATION_CREDENTIALS:-${QOSMON_GCS_CRED_FILE}}"

cargo run --quiet -- --config-dir configs/check --concurrency 50 --format plain
