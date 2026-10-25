#!/usr/bin/env bash
set -e

source run.env.sh
if [[ ! -f "${QOSMON_GCS_CRED_FILE}" ]]; then
    bwww get-attachment koishi-qosmon qosmon-gcs-upload.json >"${QOSMON_GCS_CRED_FILE}"
fi


PUBLIC_IMAGE_NAME=${PUBLIC_IMAGE_NAME:-"${QOSMON_PUBLIC_IMAGE}"}
if [[ "$PUBLIC_IMAGE_NAME" != cloudpublic/* ]]; then
  echo "Error: PUBLIC_IMAGE_NAME must start with cloudpublic/."
  exit 1
fi

GHCR_HOST=${GHCR_HOST:-"$(bwww get-password koishi.deploy.cloudpublic_registry_host)"}
GHCR_OWNER=${GHCR_OWNER:-"$(bwww get-password koishi.deploy.cloudpublic_registry_id)"}
export IMAGE_URI="${GHCR_HOST}/${GHCR_OWNER}/${PUBLIC_IMAGE_NAME#cloudpublic/}"

export PROJECT_ID=$(bwww get-password koishi.deploy.cloudprivate_registry_id)
if [ -z "$PROJECT_ID" ]; then
  echo "Error: GCP Project ID could not be determined. Please set it using 'gcloud config set project [PROJECT_ID]' or run 'gcloud auth login'."
  exit 1
fi

export RESULTS_BUCKET=$(bwww get-field koishi-qosmon gcs-bucket)
