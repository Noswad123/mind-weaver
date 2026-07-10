# Hive Sync Cloud Deployment (Cloud Run + Cloud SQL)

> Status: parked legacy Hive Sync notes. The Rust `mw` CLI no longer supports
> top-level `mw sync`; local notes use `mw notes sync` / `mw seal`. Revisit these
> commands only if Hive Sync returns under a new boundary.

This guide deploys `hive-sync-api` to Google Cloud using:

- **Cloud Run** (API runtime)
- **Cloud SQL for PostgreSQL** (persistent sync store)
- **Secret Manager** (DATABASE_URL + device tokens)
- **Artifact Registry** (container images)

## 1) Prerequisites

1. Install and authenticate `gcloud`.
2. Ensure your account can administer:
   - Cloud Run
   - Cloud SQL
   - Artifact Registry
   - Secret Manager
   - IAM
3. From the repository root, make scripts executable once:

```bash
chmod +x scripts/cloud/bootstrap-hive-sync-infra.sh scripts/cloud/deploy-hive-sync-api.sh
```

If `gcloud` reports that `PROJECT_ID` is empty, either:

- pass the literal `--project "<project-id>"` flag to commands, or
- run `gcloud config set project <project-id>`

## 2) Bootstrap cloud resources

At minimum set `PROJECT_ID` and run:

```bash
PROJECT_ID="your-gcp-project" \
REGION="us-central1" \
scripts/cloud/bootstrap-hive-sync-infra.sh
```

This creates/ensures:

- required GCP APIs
- Artifact Registry repo (`hive-sync`)
- Cloud SQL instance (`hive-sync-pg`)
- Postgres DB + app user
- Secret Manager secrets:
  - `hive-sync-api-database-url`
  - `hive-sync-api-device-tokens` (placeholder by default)
- runtime service account (`hive-sync-api-runner@<project>.iam.gserviceaccount.com`)
- IAM grants for Cloud SQL + Secret Manager access

### Important after bootstrap

If you keep auth enabled, rotate placeholder device tokens immediately:

```bash
printf '%s' "desktop=<real-token>,phone=<real-token>" \
  | gcloud secrets versions add hive-sync-api-device-tokens --data-file=- --project "$PROJECT_ID"
```

For full rotation procedure, see:

- `docs/hive-sync-token-rotation-checklist.md`

### Verify token mapping (read-only)

```bash
gcloud secrets versions access latest \
  --secret "hive-sync-api-device-tokens" \
  --project "$PROJECT_ID"
```

If output is `desktop=abc123,phone=def456`, then use:

- `desktop` as `--device-id`
- `abc123` as bearer token value

Do **not** pass `desktop=abc123` as bearer token.

If you add a new device, write a full replacement map that includes **all** devices (for example
`work=...,personal=...,phone=...`). A new secret version replaces the active mapping; it does not
merge automatically with older versions.

Quick validation:

```bash
mw sync token check \
  --endpoint "https://<service-url>" \
  --device-id "desktop" \
  --token "abc123"
```

## 3) Deploy service to Cloud Run

```bash
PROJECT_ID="your-gcp-project" \
REGION="us-central1" \
CLOUD_SQL_INSTANCE="hive-sync-pg" \
scripts/cloud/deploy-hive-sync-api.sh
```

The deploy script:

1. builds/pushes container image using `Dockerfile.hive-sync-api`
2. deploys Cloud Run service
3. attaches Cloud SQL instance
4. injects secrets as runtime env vars
5. sets `HIVE_SYNC_REQUIRE_AUTH` (default `true`)

## 4) Verify deployment

Use the service URL printed by deploy script:

```bash
curl "https://<service-url>/healthz"
```

Optional remote doctor check (token required if auth enabled):

```bash
mw sync doctor --endpoint "https://<service-url>" --token "<device-token>"
```

Safer local storage/fetch on macOS (Keychain):

```bash
printf '%s' '<device-token>' | mw sync token store --token-stdin --device-id desktop
mw sync doctor --endpoint "https://<service-url>" --device-id desktop --token-from-keychain
```

Alternative (dynamic fetch via Secret Manager, no local persistence):

```bash
mw sync doctor \
  --endpoint "https://<service-url>" \
  --token-command "gcloud secrets versions access latest --secret hive-sync-api-device-tokens --project $PROJECT_ID | tr ',' '\\n' | awk -F= '$1==\"desktop\"{print $2}'"
```

## 5) Client usage against cloud endpoint

```bash
mw sync \
  --endpoint "https://<service-url>" \
  --device-id "desktop" \
  --token "<device-token>"
```

Or via Keychain on macOS:

```bash
mw sync \
  --endpoint "https://<service-url>" \
  --device-id "desktop" \
  --token-from-keychain
```

## 6) Common overrides

Both scripts support env overrides:

- `PROJECT_ID`
- `REGION`
- `CLOUD_SQL_INSTANCE`
- `RUN_SA_NAME`
- `ARTIFACT_REPOSITORY`
- `DB_URL_SECRET_NAME`
- `DEVICE_TOKENS_SECRET_NAME`

Additional bootstrap-only:

- `CLOUD_SQL_DATABASE_VERSION` (default `POSTGRES_15`)
- `CLOUD_SQL_TIER` (default `db-custom-1-3840`)
- `CLOUD_SQL_STORAGE_GB` (default `20`)
- `CLOUD_SQL_DB_NAME` (default `hive_sync`)
- `CLOUD_SQL_DB_USER` (default `hive_sync_app`)
- `DB_PASSWORD` (auto-generated if omitted)

Additional deploy-only:

- `SERVICE_NAME` (default `hive-sync-api`)
- `IMAGE_NAME` / `IMAGE_TAG`
- `REQUIRE_AUTH` (default `true`)
- `CORS_ALLOWED_ORIGINS` (optional; `;` or `,` separated origin list, e.g. `https://pwa.example.com;http://localhost:5173`)
- `ALLOW_UNAUTHENTICATED` (default `true`)
- `MIN_INSTANCES` / `MAX_INSTANCES`

### Browser/PWA note

If PWA and API are on different origins, set `CORS_ALLOWED_ORIGINS` during deploy.

Example:

```bash
PROJECT_ID="your-gcp-project" \
REGION="us-east1" \
CLOUD_SQL_INSTANCE="hive-sync-pg" \
CORS_ALLOWED_ORIGINS="https://hive-pwa-abc-ue.a.run.app;http://localhost:5173" \
scripts/cloud/deploy-hive-sync-api.sh
```

For LAN phone testing against the local Vite dev server, include the machine's LAN origin too:

```bash
PROJECT_ID="your-gcp-project" \
REGION="us-east1" \
CLOUD_SQL_INSTANCE="hive-sync-pg" \
CORS_ALLOWED_ORIGINS="http://localhost:5173;http://<lan-ip>:5173" \
bash scripts/cloud/deploy-hive-sync-api.sh
```

Remember:

- `http://<lan-ip>:5173` is the **PWA URL**
- `https://<service-url>` is the **API endpoint** entered into the app

## 7) Rollback (quick)

List revisions:

```bash
gcloud run revisions list --service hive-sync-api --region "$REGION" --project "$PROJECT_ID"
```

Send traffic to previous revision (example):

```bash
gcloud run services update-traffic hive-sync-api \
  --region "$REGION" \
  --project "$PROJECT_ID" \
  --to-revisions "hive-sync-api-00009-abc=100"
```

## 8) Next operational hardening

After initial deployment, set up:

- monitoring and alerting: `docs/hive-sync-monitoring.md`
- backup and recovery baseline: `docs/hive-sync-backup-recovery.md`
