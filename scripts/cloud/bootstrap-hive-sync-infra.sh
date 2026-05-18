#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-us-central1}"

ARTIFACT_REPOSITORY="${ARTIFACT_REPOSITORY:-hive-sync}"

CLOUD_SQL_INSTANCE="${CLOUD_SQL_INSTANCE:-hive-sync-pg}"
CLOUD_SQL_DATABASE_VERSION="${CLOUD_SQL_DATABASE_VERSION:-POSTGRES_15}"
CLOUD_SQL_TIER="${CLOUD_SQL_TIER:-db-custom-1-3840}"
CLOUD_SQL_STORAGE_GB="${CLOUD_SQL_STORAGE_GB:-20}"
CLOUD_SQL_DB_NAME="${CLOUD_SQL_DB_NAME:-hive_sync}"
CLOUD_SQL_DB_USER="${CLOUD_SQL_DB_USER:-hive_sync_app}"

RUN_SA_NAME="${RUN_SA_NAME:-hive-sync-api-runner}"

DB_URL_SECRET_NAME="${DB_URL_SECRET_NAME:-hive-sync-api-database-url}"
DEVICE_TOKENS_SECRET_NAME="${DEVICE_TOKENS_SECRET_NAME:-hive-sync-api-device-tokens}"

CREATE_DEVICE_TOKENS_SECRET="${CREATE_DEVICE_TOKENS_SECRET:-true}"
DEVICE_TOKENS_VALUE="${DEVICE_TOKENS_VALUE:-desktop=replace-me}"

generate_db_password() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16
    return 0
  fi

  # Fallback path when openssl is unavailable.
  # Avoid pipefail/sigpipe aborts from tr|head by temporarily disabling pipefail.
  set +o pipefail
  local generated
  generated="$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32 || true)"
  set -o pipefail

  if [[ -z "${generated}" ]]; then
    generated="sync$(date +%s)"
  fi

  printf '%s' "${generated}"
}

wait_for_service_account() {
  local email="$1"
  local attempts="${2:-20}"
  local sleep_seconds="${3:-3}"

  for ((i = 1; i <= attempts; i++)); do
    if gcloud iam service-accounts describe "${email}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${sleep_seconds}"
  done

  return 1
}

if [[ -z "${PROJECT_ID}" ]]; then
  echo "❌ PROJECT_ID is required"
  echo "   Example: PROJECT_ID=my-gcp-project scripts/cloud/bootstrap-hive-sync-infra.sh"
  exit 1
fi

if ! command -v gcloud >/dev/null 2>&1; then
  echo "❌ gcloud CLI is required"
  exit 1
fi

echo "🔧 Project: ${PROJECT_ID}"
echo "🔧 Region:  ${REGION}"

gcloud config set project "${PROJECT_ID}" >/dev/null

echo "🔌 Enabling required Google Cloud APIs..."
gcloud services enable \
  run.googleapis.com \
  sqladmin.googleapis.com \
  artifactregistry.googleapis.com \
  cloudbuild.googleapis.com \
  secretmanager.googleapis.com \
  iam.googleapis.com \
  --project "${PROJECT_ID}"

echo "📦 Ensuring Artifact Registry repository (${ARTIFACT_REPOSITORY}) exists..."
if ! gcloud artifacts repositories describe "${ARTIFACT_REPOSITORY}" \
  --location "${REGION}" \
  --project "${PROJECT_ID}" >/dev/null 2>&1; then
  gcloud artifacts repositories create "${ARTIFACT_REPOSITORY}" \
    --repository-format=docker \
    --location "${REGION}" \
    --description="Hive sync container images" \
    --project "${PROJECT_ID}"
fi

echo "🗄️  Ensuring Cloud SQL instance (${CLOUD_SQL_INSTANCE}) exists..."
if ! gcloud sql instances describe "${CLOUD_SQL_INSTANCE}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
  gcloud sql instances create "${CLOUD_SQL_INSTANCE}" \
    --database-version "${CLOUD_SQL_DATABASE_VERSION}" \
    --tier "${CLOUD_SQL_TIER}" \
    --storage-size "${CLOUD_SQL_STORAGE_GB}" \
    --region "${REGION}" \
    --project "${PROJECT_ID}"
fi

echo "🗄️  Ensuring database (${CLOUD_SQL_DB_NAME}) exists..."
if ! gcloud sql databases describe "${CLOUD_SQL_DB_NAME}" \
  --instance "${CLOUD_SQL_INSTANCE}" \
  --project "${PROJECT_ID}" >/dev/null 2>&1; then
  gcloud sql databases create "${CLOUD_SQL_DB_NAME}" \
    --instance "${CLOUD_SQL_INSTANCE}" \
    --project "${PROJECT_ID}"
fi

if [[ -z "${DB_PASSWORD:-}" ]]; then
  DB_PASSWORD="$(generate_db_password)"
fi

echo "👤 Creating/updating Cloud SQL user (${CLOUD_SQL_DB_USER})..."
if ! gcloud sql users create "${CLOUD_SQL_DB_USER}" \
  --instance "${CLOUD_SQL_INSTANCE}" \
  --password "${DB_PASSWORD}" \
  --project "${PROJECT_ID}" >/dev/null 2>&1; then
  gcloud sql users set-password "${CLOUD_SQL_DB_USER}" \
    --instance "${CLOUD_SQL_INSTANCE}" \
    --password "${DB_PASSWORD}" \
    --project "${PROJECT_ID}"
fi

INSTANCE_CONNECTION_NAME="$(gcloud sql instances describe "${CLOUD_SQL_INSTANCE}" --project "${PROJECT_ID}" --format='value(connectionName)')"
DATABASE_URL_VALUE="user=${CLOUD_SQL_DB_USER} password=${DB_PASSWORD} dbname=${CLOUD_SQL_DB_NAME} host=/cloudsql/${INSTANCE_CONNECTION_NAME} sslmode=disable"

upsert_secret() {
  local name="$1"
  local value="$2"

  if gcloud secrets describe "${name}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
    printf '%s' "${value}" | gcloud secrets versions add "${name}" --data-file=- --project "${PROJECT_ID}" >/dev/null
  else
    printf '%s' "${value}" | gcloud secrets create "${name}" --replication-policy=automatic --data-file=- --project "${PROJECT_ID}" >/dev/null
  fi
}

echo "🔐 Creating/updating DATABASE_URL secret (${DB_URL_SECRET_NAME})..."
upsert_secret "${DB_URL_SECRET_NAME}" "${DATABASE_URL_VALUE}"

if [[ "${CREATE_DEVICE_TOKENS_SECRET}" == "true" ]]; then
  echo "🔐 Creating/updating device tokens secret (${DEVICE_TOKENS_SECRET_NAME})..."
  upsert_secret "${DEVICE_TOKENS_SECRET_NAME}" "${DEVICE_TOKENS_VALUE}"
fi

RUN_SA_EMAIL="${RUN_SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

echo "🪪 Ensuring runtime service account (${RUN_SA_EMAIL}) exists..."
if ! gcloud iam service-accounts describe "${RUN_SA_EMAIL}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
  gcloud iam service-accounts create "${RUN_SA_NAME}" \
    --display-name="Hive Sync API runtime" \
    --project "${PROJECT_ID}"
fi

if ! wait_for_service_account "${RUN_SA_EMAIL}" 20 3; then
  echo "❌ Runtime service account did not propagate in time: ${RUN_SA_EMAIL}"
  echo "   Re-run this script in ~1 minute."
  exit 1
fi

echo "🔑 Granting runtime IAM permissions..."
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${RUN_SA_EMAIL}" \
  --role "roles/cloudsql.client" >/dev/null

for secret_name in "${DB_URL_SECRET_NAME}" "${DEVICE_TOKENS_SECRET_NAME}"; do
  if gcloud secrets describe "${secret_name}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
    gcloud secrets add-iam-policy-binding "${secret_name}" \
      --member "serviceAccount:${RUN_SA_EMAIL}" \
      --role "roles/secretmanager.secretAccessor" \
      --project "${PROJECT_ID}" >/dev/null
  fi
done

echo
echo "✅ Cloud bootstrap complete"
echo "Project:                 ${PROJECT_ID}"
echo "Region:                  ${REGION}"
echo "Artifact repository:     ${ARTIFACT_REPOSITORY}"
echo "Cloud SQL instance:      ${CLOUD_SQL_INSTANCE}"
echo "Cloud SQL connection:    ${INSTANCE_CONNECTION_NAME}"
echo "Cloud SQL database:      ${CLOUD_SQL_DB_NAME}"
echo "Cloud SQL user:          ${CLOUD_SQL_DB_USER}"
echo "DATABASE_URL secret:     ${DB_URL_SECRET_NAME}"
if gcloud secrets describe "${DEVICE_TOKENS_SECRET_NAME}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
  echo "Device tokens secret:    ${DEVICE_TOKENS_SECRET_NAME}"
fi
echo "Runtime service account: ${RUN_SA_EMAIL}"
echo
echo "Next step:"
echo "  PROJECT_ID=${PROJECT_ID} REGION=${REGION} CLOUD_SQL_INSTANCE=${CLOUD_SQL_INSTANCE} scripts/cloud/deploy-hive-sync-api.sh"
