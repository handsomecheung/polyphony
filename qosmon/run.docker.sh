#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

source run.env.sh
bash run.common.sh

sudo docker run --rm \
  -e "GOOGLE_APPLICATION_CREDENTIALS=${QOSMON_GCS_CRED_FILE}" \
  -v "$(pwd)/configs:/app/configs:ro" \
  -v "${QOSMON_GCS_CRED_FILE}:${QOSMON_GCS_CRED_FILE}:ro" \
  qosmon:latest
