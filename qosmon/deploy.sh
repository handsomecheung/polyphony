#!/usr/bin/env bash
set -e

PROJECT_ID=$(gcloud config get-value project 2>/dev/null)
REGION=${REGION:-"asia-northeast1"}
REPO_NAME=${REPO_NAME:-"cloudrun"}
IMAGE_NAME=${IMAGE_NAME:-"qosmon"}
JOB_NAME=${JOB_NAME:-"qosmon-job"}
SCHEDULER_JOB_NAME=${SCHEDULER_JOB_NAME:-"qosmon-scheduler"}
SERVICE_ACCOUNT_NAME=${SERVICE_ACCOUNT_NAME:-"qosmon-scheduler-sa"}

if [ -z "$PROJECT_ID" ]; then
  echo "Error: GCP Project ID could not be determined. Please set it using 'gcloud config set project [PROJECT_ID]' or run 'gcloud auth login'."
  exit 1
fi

echo "============================================="
echo " Deploying qosmon to Cloud Run Jobs"
echo " Project:          $PROJECT_ID"
echo " Region:           $REGION"
echo " Repository:       $REPO_NAME"
echo " Image Name:       $IMAGE_NAME"
echo " Job Name:         $JOB_NAME"
echo " Scheduler Job:    $SCHEDULER_JOB_NAME"
echo "============================================="

echo "Enabling required APIs..."
gcloud services enable \
  artifactregistry.googleapis.com \
  run.googleapis.com \
  cloudbuild.googleapis.com \
  cloudscheduler.googleapis.com \
  --project="$PROJECT_ID"

echo "Checking Artifact Registry Repository..."
if ! gcloud artifacts repositories describe "$REPO_NAME" --location="$REGION" --project="$PROJECT_ID" >/dev/null 2>&1; then
  echo "Creating Artifact Registry repository '$REPO_NAME'..."
  gcloud artifacts repositories create "$REPO_NAME" \
    --repository-format=docker \
    --location="$REGION" \
    --description="Docker repository for CloudRun" \
    --project="$PROJECT_ID"
else
  echo "Repository '$REPO_NAME' already exists."
fi

IMAGE_URI="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO_NAME}/${IMAGE_NAME}:latest"
echo "Building and pushing Docker image to $IMAGE_URI..."
gcloud builds submit --tag "$IMAGE_URI" --project="$PROJECT_ID" .

echo "Deploying Cloud Run Job '$JOB_NAME'..."
gcloud run jobs deploy "$JOB_NAME" \
  --image="$IMAGE_URI" \
  --region="$REGION" \
  --project="$PROJECT_ID" \
  --max-retries=0 \
  --task-timeout="5m"

SA_EMAIL="${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
echo "Setting up Service Account: $SA_EMAIL..."
if ! gcloud iam service-accounts describe "$SA_EMAIL" --project="$PROJECT_ID" >/dev/null 2>&1; then
  echo "Creating service account..."
  gcloud iam service-accounts create "$SERVICE_ACCOUNT_NAME" \
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
    --schedule="*/10 * * * *" \
    --uri="$SCHEDULER_URI" \
    --http-method=POST \
    --oauth-service-account-email="$SA_EMAIL" \
    --oauth-token-scope="https://www.googleapis.com/auth/cloud-platform" \
    --project="$PROJECT_ID"
else
  echo "Creating new Cloud Scheduler Job '$SCHEDULER_JOB_NAME'..."
  gcloud scheduler jobs create http "$SCHEDULER_JOB_NAME" \
    --location="$REGION" \
    --schedule="*/10 * * * *" \
    --uri="$SCHEDULER_URI" \
    --http-method=POST \
    --oauth-service-account-email="$SA_EMAIL" \
    --oauth-token-scope="https://www.googleapis.com/auth/cloud-platform" \
    --project="$PROJECT_ID"
fi

echo "============================================="
echo " Deployment completed successfully!"
echo " The monitoring job is scheduled to run every 10 minutes."
echo "============================================="
