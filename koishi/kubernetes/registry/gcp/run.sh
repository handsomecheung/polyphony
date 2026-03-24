#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

# run the following command to set the project and location for gcloud
# gcloud config set project ${project_id}
# gcloud config set artifacts/location ${location}

gcloud artifacts repositories set-cleanup-policies docker \
    --policy=cleanup-cache.json \
    --no-dry-run
