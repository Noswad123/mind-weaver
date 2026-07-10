# Hive Sync Monitoring & Alerts

> Status: parked legacy Hive Sync notes. The Rust `mw` CLI no longer supports
> top-level `mw sync`; local notes use `mw notes sync` / `mw seal`. Revisit these
> commands only if Hive Sync returns under a new boundary.

This guide configures basic production monitoring for `hive-sync-api` on Cloud Run.

## What this sets up

Using `scripts/cloud/setup-hive-sync-monitoring.sh`, we configure:

1. **Uptime check** for `https://<service-url>/healthz`
2. **Alert policy** when uptime check fails for ~5 minutes
3. **Alert policy** when Cloud Run 5xx request rate is elevated for ~5 minutes

## 1) Run monitoring setup

```bash
PROJECT_ID="your-gcp-project" \
REGION="us-east1" \
SERVICE_NAME="hive-sync-api" \
SERVICE_URL="https://<service-url>" \
scripts/cloud/setup-hive-sync-monitoring.sh
```

Optional notification channels:

```bash
NOTIFICATION_CHANNELS="projects/<project>/notificationChannels/<id>" \
PROJECT_ID="your-gcp-project" \
REGION="us-east1" \
SERVICE_NAME="hive-sync-api" \
SERVICE_URL="https://<service-url>" \
scripts/cloud/setup-hive-sync-monitoring.sh
```

If no channel is provided, policies are still created but won’t notify anyone.

## 2) Verify created resources

```bash
gcloud monitoring uptime list-configs --project "$PROJECT_ID"
gcloud monitoring policies list --project "$PROJECT_ID"
```

## 3) Operate with `mw sync doctor`

For local and remote sync health checks:

```bash
mw sync doctor \
  --endpoint "https://<service-url>" \
  --device-id "desktop" \
  --token-from-keychain
```

## 4) Two-computer sync readiness

Yes — at this point you are ready for eventual two-computer sync.

Requirements:

- Both computers use same `--endpoint`
- Each computer has unique `--device-id`
- Each computer has valid bearer token for that device
- Each computer runs periodic `mw sync`

Example cadence (every 5 minutes) on each machine:

```bash
*/5 * * * * /path/to/mw sync --endpoint "https://<service-url>" --device-id "desktop" --token-from-keychain >> /tmp/mw-sync.log 2>&1
```

Eventual consistency notes:

- Devices converge as long as they reconnect and continue running sync cycles.
- Conflict handling is deterministic (LWW v1) and conflict artifacts are retained locally.
