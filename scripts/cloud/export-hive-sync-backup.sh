#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-}"
CLOUD_SQL_INSTANCE="${CLOUD_SQL_INSTANCE:-hive-sync-pg}"
CLOUD_SQL_DB_NAME="${CLOUD_SQL_DB_NAME:-hive_sync}"
BACKUP_BUCKET="${BACKUP_BUCKET:-${PROJECT_ID}-hive-sync-backups}"
BACKUP_PREFIX="${BACKUP_PREFIX:-sql-exports}"

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
FILENAME="${CLOUD_SQL_INSTANCE}-${CLOUD_SQL_DB_NAME}-${TIMESTAMP}.sql.gz"
DEST_URI="gs://${BACKUP_BUCKET}/${BACKUP_PREFIX}/${FILENAME}"

if [[ -z "${PROJECT_ID}" ]]; then
  echo "❌ PROJECT_ID is required"
  exit 1
fi

if ! command -v gcloud >/dev/null 2>&1; then
  echo "❌ gcloud CLI is required"
  exit 1
fi

gcloud config set project "${PROJECT_ID}" >/dev/null

echo "🔧 Project:           ${PROJECT_ID}"
echo "🔧 Cloud SQL instance:${CLOUD_SQL_INSTANCE}"
echo "🔧 Database:          ${CLOUD_SQL_DB_NAME}"
echo "🔧 Destination:       ${DEST_URI}"

echo "📤 Starting Cloud SQL export..."
gcloud sql export sql "${CLOUD_SQL_INSTANCE}" "${DEST_URI}" \
  --project "${PROJECT_ID}" \
  --database "${CLOUD_SQL_DB_NAME}" \
  --offload

echo
echo "✅ Export completed"
echo "Backup file: ${DEST_URI}"
echo
echo "Restore command example:"
echo "  gcloud sql import sql ${CLOUD_SQL_INSTANCE} ${DEST_URI} --database=${CLOUD_SQL_DB_NAME} --project=${PROJECT_ID}"
