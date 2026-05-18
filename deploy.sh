#!/usr/bin/env bash
set -euo pipefail

: "${PROJECT_ID:?Set PROJECT_ID}"
: "${REGION:?Set REGION}"
: "${CLOUD_SQL_INSTANCE:?Set CLOUD_SQL_INSTANCE}"
: "${CORS_ALLOWED_ORIGINS:?Set CORS_ALLOWED_ORIGINS}"

bash scripts/cloud/deploy-hive-sync-api.sh
