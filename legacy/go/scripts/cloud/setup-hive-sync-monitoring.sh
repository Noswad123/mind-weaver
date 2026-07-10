#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-us-east1}"
SERVICE_NAME="${SERVICE_NAME:-hive-sync-api}"
SERVICE_URL="${SERVICE_URL:-}"

UPTIME_DISPLAY_NAME="${UPTIME_DISPLAY_NAME:-hive-sync-api-healthz}"
UPTIME_PATH="${UPTIME_PATH:-/healthz}"
UPTIME_PERIOD_MINUTES="${UPTIME_PERIOD_MINUTES:-1}"
UPTIME_TIMEOUT_SECONDS="${UPTIME_TIMEOUT_SECONDS:-10}"

ALERT_UPTIME_DISPLAY_NAME="${ALERT_UPTIME_DISPLAY_NAME:-hive-sync-api-uptime-failing}"
ALERT_5XX_DISPLAY_NAME="${ALERT_5XX_DISPLAY_NAME:-hive-sync-api-5xx-rate}"
ALERT_5XX_RATE_THRESHOLD="${ALERT_5XX_RATE_THRESHOLD:-0.05}"

NOTIFICATION_CHANNELS="${NOTIFICATION_CHANNELS:-}"

if [[ -z "${PROJECT_ID}" ]]; then
  echo "❌ PROJECT_ID is required"
  exit 1
fi

if [[ -z "${SERVICE_URL}" ]]; then
  echo "❌ SERVICE_URL is required"
  echo "   Example: SERVICE_URL=https://hive-sync-api-abc-ue.a.run.app"
  exit 1
fi

if ! command -v gcloud >/dev/null 2>&1; then
  echo "❌ gcloud CLI is required"
  exit 1
fi

gcloud config set project "${PROJECT_ID}" >/dev/null

extract_host() {
  local raw="$1"
  raw="${raw#http://}"
  raw="${raw#https://}"
  raw="${raw%%/*}"
  printf '%s' "${raw}"
}

json_array_from_csv() {
  local csv="$1"
  if [[ -z "${csv// }" ]]; then
    printf '[]'
    return 0
  fi

  local out="["
  local first=1
  IFS=',' read -r -a parts <<<"${csv}"
  for part in "${parts[@]}"; do
    local item
    item="$(printf '%s' "${part}" | xargs)"
    if [[ -z "${item}" ]]; then
      continue
    fi
    if [[ ${first} -eq 0 ]]; then
      out+=","
    fi
    out+="\"${item}\""
    first=0
  done
  out+="]"
  printf '%s' "${out}"
}

SERVICE_HOST="$(extract_host "${SERVICE_URL}")"
if [[ -z "${SERVICE_HOST}" ]]; then
  echo "❌ could not extract host from SERVICE_URL=${SERVICE_URL}"
  exit 1
fi

echo "🔧 Project:       ${PROJECT_ID}"
echo "🔧 Region:        ${REGION}"
echo "🔧 Service name:  ${SERVICE_NAME}"
echo "🔧 Service URL:   ${SERVICE_URL}"
echo "🔧 Service host:  ${SERVICE_HOST}"

echo "🔌 Enabling monitoring API..."
gcloud services enable monitoring.googleapis.com --project "${PROJECT_ID}" >/dev/null

echo "🩺 Ensuring uptime check (${UPTIME_DISPLAY_NAME}) exists..."
UPTIME_NAME="$(gcloud monitoring uptime list-configs \
  --project "${PROJECT_ID}" \
  --filter "display_name=\"${UPTIME_DISPLAY_NAME}\"" \
  --format 'value(name)' \
  --limit 1)"

if [[ -z "${UPTIME_NAME}" ]]; then
  gcloud monitoring uptime create "${UPTIME_DISPLAY_NAME}" \
    --project "${PROJECT_ID}" \
    --resource-type uptime-url \
    --resource-labels "host=${SERVICE_HOST},project_id=${PROJECT_ID}" \
    --path "${UPTIME_PATH}" \
    --protocol https \
    --request-method get \
    --validate-ssl=true \
    --period "${UPTIME_PERIOD_MINUTES}" \
    --timeout "${UPTIME_TIMEOUT_SECONDS}" \
    --regions "usa-iowa,usa-oregon,usa-virginia" \
    --user-labels "app=hive-sync,service=${SERVICE_NAME}" \
    >/dev/null

  UPTIME_NAME="$(gcloud monitoring uptime list-configs \
    --project "${PROJECT_ID}" \
    --filter "display_name=\"${UPTIME_DISPLAY_NAME}\"" \
    --format 'value(name)' \
    --limit 1)"
else
  echo "   existing uptime check found: ${UPTIME_NAME}"
fi

if [[ -z "${UPTIME_NAME}" ]]; then
  echo "❌ failed to resolve uptime check config name"
  exit 1
fi

UPTIME_ID="${UPTIME_NAME##*/}"
CHANNELS_JSON="$(json_array_from_csv "${NOTIFICATION_CHANNELS}")"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

UPTIME_POLICY_FILE="${TMP_DIR}/uptime-policy.json"
cat >"${UPTIME_POLICY_FILE}" <<EOF
{
  "displayName": "${ALERT_UPTIME_DISPLAY_NAME}",
  "combiner": "OR",
  "enabled": true,
  "notificationChannels": ${CHANNELS_JSON},
  "documentation": {
    "content": "Hive Sync API health check is failing.\\n\\nRunbook: docs/hive-sync-runbook.md",
    "mimeType": "text/markdown"
  },
  "conditions": [
    {
      "displayName": "uptime check failed",
      "conditionThreshold": {
        "filter": "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND resource.type=\"uptime_url\" AND metric.label.check_id=\"${UPTIME_ID}\"",
        "comparison": "COMPARISON_LT",
        "thresholdValue": 1,
        "duration": "300s",
        "aggregations": [
          {
            "alignmentPeriod": "60s",
            "perSeriesAligner": "ALIGN_NEXT_OLDER"
          }
        ],
        "trigger": {
          "count": 1
        }
      }
    }
  ]
}
EOF

FIVEXX_POLICY_FILE="${TMP_DIR}/5xx-policy.json"
cat >"${FIVEXX_POLICY_FILE}" <<EOF
{
  "displayName": "${ALERT_5XX_DISPLAY_NAME}",
  "combiner": "OR",
  "enabled": true,
  "notificationChannels": ${CHANNELS_JSON},
  "documentation": {
    "content": "Hive Sync API Cloud Run 5xx request rate is elevated.\\n\\nRunbook: docs/hive-sync-runbook.md",
    "mimeType": "text/markdown"
  },
  "conditions": [
    {
      "displayName": "cloud run 5xx request rate",
      "conditionThreshold": {
        "filter": "metric.type=\"run.googleapis.com/request_count\" AND resource.type=\"cloud_run_revision\" AND resource.label.service_name=\"${SERVICE_NAME}\" AND resource.label.location=\"${REGION}\" AND metric.label.response_code_class=\"5xx\"",
        "comparison": "COMPARISON_GT",
        "thresholdValue": ${ALERT_5XX_RATE_THRESHOLD},
        "duration": "300s",
        "aggregations": [
          {
            "alignmentPeriod": "60s",
            "perSeriesAligner": "ALIGN_RATE"
          }
        ],
        "trigger": {
          "count": 1
        }
      }
    }
  ]
}
EOF

upsert_policy() {
  local display_name="$1"
  local policy_file="$2"

  local existing
  existing="$(gcloud monitoring policies list \
    --project "${PROJECT_ID}" \
    --filter "display_name=\"${display_name}\"" \
    --format 'value(name)' \
    --limit 1)"

  if [[ -z "${existing}" ]]; then
    echo "🔔 Creating alert policy: ${display_name}"
    gcloud monitoring policies create \
      --project "${PROJECT_ID}" \
      --policy-from-file "${policy_file}" >/dev/null
  else
    echo "🔔 Updating alert policy: ${display_name}"
    gcloud monitoring policies update "${existing}" \
      --project "${PROJECT_ID}" \
      --policy-from-file "${policy_file}" >/dev/null
  fi
}

upsert_policy "${ALERT_UPTIME_DISPLAY_NAME}" "${UPTIME_POLICY_FILE}"
upsert_policy "${ALERT_5XX_DISPLAY_NAME}" "${FIVEXX_POLICY_FILE}"

echo
echo "✅ Monitoring setup complete"
echo "Uptime check: ${UPTIME_NAME}"
echo "Alert policy: ${ALERT_UPTIME_DISPLAY_NAME}"
echo "Alert policy: ${ALERT_5XX_DISPLAY_NAME}"
if [[ "${CHANNELS_JSON}" == "[]" ]]; then
  echo "⚠️  No notification channels configured. Set NOTIFICATION_CHANNELS to receive alerts."
fi
