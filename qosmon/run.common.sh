#!/usr/bin/env bash
set -e

source run.env.sh
if [[ ! -f "${QOSMON_GCS_CRED_FILE}" ]]; then
    bwww get-attachment koishi-qosmon qosmon-gcs-upload.json >"${QOSMON_GCS_CRED_FILE}"
fi
