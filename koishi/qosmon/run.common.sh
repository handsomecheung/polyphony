#!/usr/bin/env bash
set -e

source run.env.sh

cmd_tacet=tacet-wrapper

if [[ ! -f "${QOSMON_GCS_CRED_FILE}" ]]; then
    "${cmd_tacet}" read 'tacet://*/koishi-qosmon/qosmon-gcs-upload.json' >"${QOSMON_GCS_CRED_FILE}"
fi


PUBLIC_IMAGE_NAME=${PUBLIC_IMAGE_NAME:-"${QOSMON_PUBLIC_IMAGE}"}
if [[ "$PUBLIC_IMAGE_NAME" != cloudpublic/* ]]; then
  echo "Error: PUBLIC_IMAGE_NAME must start with cloudpublic/."
  exit 1
fi

ghcr_host=$(${cmd_tacet} read 'tacet://*/koishi.deploy.cloudpublic_registry_host/password')
GHCR_HOST=${GHCR_HOST:-"${ghcr_host}"}

ghcr_owner=$(${cmd_tacet} read 'tacet://*/koishi.deploy.cloudpublic_registry_id/password')
GHCR_OWNER=${GHCR_OWNER:-"${ghcr_owner}"}

export IMAGE_URI="${GHCR_HOST}/${GHCR_OWNER}/${PUBLIC_IMAGE_NAME#cloudpublic/}"

export PROJECT_ID=$(${cmd_tacet} read 'tacet://*/koishi.deploy.cloudprivate_registry_id/password')
if [ -z "$PROJECT_ID" ]; then
  echo "Error: GCP Project ID could not be determined. Please set it using 'gcloud config set project [PROJECT_ID]' or run 'gcloud auth login'."
  exit 1
fi

export RESULTS_BUCKET=$(${cmd_tacet} read 'tacet://*/koishi-qosmon/gcs-bucket')
