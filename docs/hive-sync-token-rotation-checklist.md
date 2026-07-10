# Hive Sync Token Rotation Checklist

> Status: parked legacy Hive Sync notes. The Rust `mw` CLI no longer supports
> top-level `mw sync`; local notes use `mw notes sync` / `mw seal`. Revisit these
> commands only if Hive Sync returns under a new boundary.

Use this checklist whenever you rotate device bearer tokens.

## Scope

- Auth model: `device_id=token` mappings in Secret Manager (`hive-sync-api-device-tokens`)
- API runtime: Cloud Run reads token mappings from secret value

## 0) Inputs

Set these once in your shell:

```bash
PROJECT_ID="your-gcp-project"
REGION="us-east1"
SERVICE_URL="https://<service-url>"
```

## 1) Inspect current mapping

```bash
CURRENT_MAP="$(gcloud secrets versions access latest \
  --secret "hive-sync-api-device-tokens" \
  --project "$PROJECT_ID" | tr -d '\n')"

echo "$CURRENT_MAP"
```

Reminder: if map is `work=abc,personal=xyz`, bearer token for `work` is `abc` (not `work=abc`).

## 2) Generate replacement token(s)

Rotate one device:

```bash
NEW_WORK_TOKEN="$(openssl rand -hex 32)"
```

Rotate all devices:

```bash
NEW_WORK_TOKEN="$(openssl rand -hex 32)"
NEW_PERSONAL_TOKEN="$(openssl rand -hex 32)"
```

## 3) Write new mapping to Secret Manager

Example with two devices:

```bash
printf 'work=%s,personal=%s' "$NEW_WORK_TOKEN" "$NEW_PERSONAL_TOKEN" \
  | gcloud secrets versions add "hive-sync-api-device-tokens" \
      --data-file=- \
      --project "$PROJECT_ID"
```

## 4) Roll Cloud Run revision

```bash
PROJECT_ID="$PROJECT_ID" REGION="$REGION" scripts/cloud/deploy-hive-sync-api.sh
```

## 5) Update each machine token source

### macOS Keychain (recommended)

```bash
printf '%s' "$NEW_WORK_TOKEN" | mw sync token store --token-stdin --device-id work
printf '%s' "$NEW_PERSONAL_TOKEN" | mw sync token store --token-stdin --device-id personal
```

### Alternative: dynamic fetch (no local persistence)

Use `--token-command` against Secret Manager.

## 6) Validate each device

On each machine:

```bash
mw sync token check \
  --endpoint "$SERVICE_URL" \
  --device-id "<device-id>" \
  --token-from-keychain
```

Then run one sync cycle:

```bash
mw sync --endpoint "$SERVICE_URL" --device-id "<device-id>" --token-from-keychain
```

## 7) Post-rotation verification

```bash
mw sync doctor --endpoint "$SERVICE_URL" --device-id "<device-id>" --token-from-keychain
```

Check:

- no `401/403` auth warnings
- cursor progresses after sync

## 8) Rollback (if needed)

If rotation broke access, write a known-good previous mapping as a new secret version and redeploy Cloud Run:

```bash
printf '%s' '<known-good-device-token-map>' \
  | gcloud secrets versions add "hive-sync-api-device-tokens" \
      --data-file=- \
      --project "$PROJECT_ID"

PROJECT_ID="$PROJECT_ID" REGION="$REGION" scripts/cloud/deploy-hive-sync-api.sh
```

## 9) Safety notes

- Avoid putting raw tokens in shell history when possible (prefer stdin + Keychain).
- Keep `device_id` values stable; rotating token does not require changing `device_id`.
- Cron jobs do not auto-refresh tokens; they will fail until local token source is updated.
