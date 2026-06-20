#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

source run.env.sh
bash run.common.sh

sudo docker run --rm -v "${QOSMON_GCS_CRED_FILE}:${QOSMON_GCS_CRED_FILE}" qosmon:latest
