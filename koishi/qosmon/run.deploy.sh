#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

source run.env.sh
source run.common.sh

REGION=${REGION:-"asia-northeast1"}
JOB_NAME=${JOB_NAME:-"qosmon-job"}
SCHEDULER_JOB_NAME=${SCHEDULER_JOB_NAME:-"qosmon-scheduler"}
CONFIG_DIRECTORY=${CONFIG_DIRECTORY:-"qosmon/configs"}
RUNTIME_SERVICE_ACCOUNT_NAME=${RUNTIME_SERVICE_ACCOUNT_NAME:-"qosmon-job-sa"}
SCHEDULER_SERVICE_ACCOUNT_NAME=${SCHEDULER_SERVICE_ACCOUNT_NAME:-"${SERVICE_ACCOUNT_NAME:-qosmon-scheduler-sa}"}

echo "============================================="
echo " Deploying qosmon to Cloud Run Jobs"
echo " Project:          $PROJECT_ID"
echo " Region:           $REGION"
echo " Public Image:     $IMAGE_URI"
echo " Job Name:         $JOB_NAME"
echo " Scheduler Job:    $SCHEDULER_JOB_NAME"
echo " Config Location:  gs://$RESULTS_BUCKET/$CONFIG_DIRECTORY"
echo "============================================="

./run.generate.sh

echo "Enabling required APIs..."
gcloud services enable \
  run.googleapis.com \
  cloudscheduler.googleapis.com \
  storage.googleapis.com \
  --project="$PROJECT_ID"

RUNTIME_SA_EMAIL="${RUNTIME_SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
echo "Setting up runtime service account: $RUNTIME_SA_EMAIL..."
if ! gcloud iam service-accounts describe "$RUNTIME_SA_EMAIL" --project="$PROJECT_ID" >/dev/null 2>&1; then
  gcloud iam service-accounts create "$RUNTIME_SERVICE_ACCOUNT_NAME" \
    --display-name="Service Account for qosmon Cloud Run Job" \
    --project="$PROJECT_ID"
fi

echo "Synchronizing runtime configuration..."
gcloud storage rsync --recursive --delete-unmatched-destination-objects \
  configs/check "gs://$RESULTS_BUCKET/$CONFIG_DIRECTORY" \
  --project="$PROJECT_ID"

echo "Granting the runtime service account access to Cloud Storage..."
gcloud storage buckets add-iam-policy-binding "gs://$RESULTS_BUCKET" \
  --member="serviceAccount:${RUNTIME_SA_EMAIL}" \
  --role="roles/storage.objectUser" \
  --project="$PROJECT_ID" >/dev/null

if gcloud run jobs describe "$JOB_NAME" --region="$REGION" --project="$PROJECT_ID" --format=export 2>/dev/null | grep -q 'name: qosmon-config'; then
  echo "Replacing the existing configuration volume..."
  gcloud run jobs update "$JOB_NAME" \
    --region="$REGION" \
    --project="$PROJECT_ID" \
    --remove-volume-mount="/app/configs/check" \
    --remove-volume="qosmon-config"
fi

echo "Deploying Cloud Run Job '$JOB_NAME'..."
gcloud run jobs deploy "$JOB_NAME" \
  --image="$IMAGE_URI" \
  --region="$REGION" \
  --project="$PROJECT_ID" \
  --max-retries=0 \
  --task-timeout="5m" \
  --service-account="$RUNTIME_SA_EMAIL" \
  --add-volume="name=qosmon-config,type=cloud-storage,bucket=${RESULTS_BUCKET},readonly=true,mount-options=only-dir=${CONFIG_DIRECTORY}" \
  --add-volume-mount="volume=qosmon-config,mount-path=/app/configs/check"

SA_EMAIL="${SCHEDULER_SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
echo "Setting up Service Account: $SA_EMAIL..."
if ! gcloud iam service-accounts describe "$SA_EMAIL" --project="$PROJECT_ID" >/dev/null 2>&1; then
  echo "Creating service account..."
  gcloud iam service-accounts create "$SCHEDULER_SERVICE_ACCOUNT_NAME" \
    --display-name="Service Account for triggering qosmon Cloud Run Job" \
    --project="$PROJECT_ID"
else
  echo "Service account already exists."
fi

echo "Granting Cloud Run Developer role to Service Account..."
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/run.developer" \
  --condition=None >/dev/null

echo "Setting up Cloud Scheduler..."
SCHEDULER_URI="https://${REGION}-run.googleapis.com/v2/projects/${PROJECT_ID}/locations/${REGION}/jobs/${JOB_NAME}:run"

if gcloud scheduler jobs describe "$SCHEDULER_JOB_NAME" --location="$REGION" --project="$PROJECT_ID" >/dev/null 2>&1; then
  echo "Updating existing Cloud Scheduler Job '$SCHEDULER_JOB_NAME'..."
  gcloud scheduler jobs update http "$SCHEDULER_JOB_NAME" \
    --location="$REGION" \
    --schedule="0 */4 * * *" \
    --uri="$SCHEDULER_URI" \
    --http-method=POST \
    --oauth-service-account-email="$SA_EMAIL" \
    --oauth-token-scope="https://www.googleapis.com/auth/cloud-platform" \
    --project="$PROJECT_ID"
else
  echo "Creating new Cloud Scheduler Job '$SCHEDULER_JOB_NAME'..."
  gcloud scheduler jobs create http "$SCHEDULER_JOB_NAME" \
    --location="$REGION" \
    --schedule="0 */4 * * *" \
    --uri="$SCHEDULER_URI" \
    --http-method=POST \
    --oauth-service-account-email="$SA_EMAIL" \
    --oauth-token-scope="https://www.googleapis.com/auth/cloud-platform" \
    --project="$PROJECT_ID"
fi

echo "============================================="
echo " Deployment completed successfully!"
echo " The monitoring job is scheduled to run every 4 hours."
echo "============================================="
