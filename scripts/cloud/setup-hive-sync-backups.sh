#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-us-east1}"

CLOUD_SQL_INSTANCE="${CLOUD_SQL_INSTANCE:-hive-sync-pg}"
BACKUP_START_TIME="${BACKUP_START_TIME:-03:00}"
RETAINED_BACKUPS_COUNT="${RETAINED_BACKUPS_COUNT:-14}"
RETAINED_TRANSACTION_LOG_DAYS="${RETAINED_TRANSACTION_LOG_DAYS:-7}"

BACKUP_BUCKET="${BACKUP_BUCKET:-${PROJECT_ID}-hive-sync-backups}"
BACKUP_BUCKET_LOCATION="${BACKUP_BUCKET_LOCATION:-${REGION}}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"

if [[ -z "${PROJECT_ID}" ]]; then
  echo "❌ PROJECT_ID is required"
  exit 1
fi

if ! command -v gcloud >/dev/null 2>&1; then
  echo "❌ gcloud CLI is required"
  exit 1
fi

gcloud config set project "${PROJECT_ID}" >/dev/null

echo "🔧 Project:                    ${PROJECT_ID}"
echo "🔧 Region:                     ${REGION}"
echo "🔧 Cloud SQL instance:         ${CLOUD_SQL_INSTANCE}"
echo "🔧 Backup start time (UTC):    ${BACKUP_START_TIME}"
echo "🔧 Retained backups:           ${RETAINED_BACKUPS_COUNT}"
echo "🔧 Retained WAL days:          ${RETAINED_TRANSACTION_LOG_DAYS}"
echo "🔧 Backup bucket:              gs://${BACKUP_BUCKET}"
echo "🔧 Backup object retention:    ${BACKUP_RETENTION_DAYS} days"

echo "🔌 Enabling required APIs..."
gcloud services enable sqladmin.googleapis.com storage.googleapis.com --project "${PROJECT_ID}" >/dev/null

echo "🗄️  Enabling Cloud SQL automated backups + PITR..."
gcloud sql instances patch "${CLOUD_SQL_INSTANCE}" \
  --project "${PROJECT_ID}" \
  --backup-start-time "${BACKUP_START_TIME}" \
  --retained-backups-count "${RETAINED_BACKUPS_COUNT}" \
  --enable-point-in-time-recovery \
  --retained-transaction-log-days "${RETAINED_TRANSACTION_LOG_DAYS}" \
  --retain-backups-on-delete \
  --quiet >/dev/null

echo "🪣 Ensuring backup export bucket exists..."
if ! gcloud storage buckets describe "gs://${BACKUP_BUCKET}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
  gcloud storage buckets create "gs://${BACKUP_BUCKET}" \
    --project "${PROJECT_ID}" \
    --location "${BACKUP_BUCKET_LOCATION}" \
    --uniform-bucket-level-access >/dev/null
fi

CLOUD_SQL_SERVICE_ACCOUNT="$(gcloud sql instances describe "${CLOUD_SQL_INSTANCE}" --project "${PROJECT_ID}" --format='value(serviceAccountEmailAddress)')"
if [[ -z "${CLOUD_SQL_SERVICE_ACCOUNT}" ]]; then
  echo "❌ Could not resolve Cloud SQL service account for instance ${CLOUD_SQL_INSTANCE}"
  exit 1
fi

echo "🔑 Granting bucket write access to Cloud SQL service account..."
gcloud storage buckets add-iam-policy-binding "gs://${BACKUP_BUCKET}" \
  --project "${PROJECT_ID}" \
  --member "serviceAccount:${CLOUD_SQL_SERVICE_ACCOUNT}" \
  --role "roles/storage.objectAdmin" >/dev/null

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

LIFECYCLE_FILE="${TMP_DIR}/lifecycle.json"
cat >"${LIFECYCLE_FILE}" <<EOF
{
  "rule": [
    {
      "action": {"type": "Delete"},
      "condition": {"age": ${BACKUP_RETENTION_DAYS}}
    }
  ]
}
EOF

echo "🧹 Applying lifecycle retention policy to backup bucket..."
gcloud storage buckets update "gs://${BACKUP_BUCKET}" \
  --project "${PROJECT_ID}" \
  --lifecycle-file "${LIFECYCLE_FILE}" >/dev/null

echo
echo "✅ Backup baseline setup complete"
echo "Cloud SQL automated backups and PITR are enabled."
echo "Backup export bucket: gs://${BACKUP_BUCKET}"
echo
echo "Next step (run manual export now):"
echo "  PROJECT_ID=${PROJECT_ID} CLOUD_SQL_INSTANCE=${CLOUD_SQL_INSTANCE} BACKUP_BUCKET=${BACKUP_BUCKET} scripts/cloud/export-hive-sync-backup.sh"
