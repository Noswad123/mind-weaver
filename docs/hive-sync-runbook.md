# Hive Sync Runbook (MVP)

This runbook covers day-to-day operation for `hive-sync-api` and `mw sync`.

## 1) Start the sync API locally

```bash
go run ./cmd/hiveSyncAPI
```

Optional auth-hardening env:

```bash
export HIVE_SYNC_REQUIRE_AUTH=true
export HIVE_SYNC_DEVICE_TOKENS="desktop=token-desktop,phone=token-phone"
export HIVE_SYNC_CORS_ALLOWED_ORIGINS="http://localhost:5173"
```

## 2) Run one bounded sync cycle

```bash
go run ./cmd/mw sync \
  --endpoint http://127.0.0.1:8080 \
  --device-id desktop \
  --token token-desktop
```

Expected observability logs include:
- push/pull counters
- conflict counts + conflict rate
- local/server cursor + lag

## 3) Run bounded worker mode

```bash
go run ./cmd/mw sync \
  --endpoint http://127.0.0.1:8080 \
  --device-id desktop \
  --token token-desktop \
  --worker-iterations 12 \
  --worker-interval 10s \
  --retry-attempts 4 \
  --retry-base-delay 500ms \
  --retry-max-delay 8s
```

This runs a finite worker loop and exits after the requested iteration count.

### Token input options for `mw sync` and `mw sync doctor`

You can provide bearer tokens in four ways:

1. `--token <value>`
2. `HIVE_SYNC_TOKEN` env var
3. `--token-command '<shell command that prints token>'`
4. `--token-from-keychain` (macOS Keychain)

If you use Keychain mode:

- default service: `mw/hive-sync`
- default account: `--device-id`

Store token in Keychain safely from stdin:

```bash
printf '%s' '<token-value>' | mw sync token store --token-stdin --device-id desktop
```

Use token from Keychain:

```bash
mw sync --endpoint https://<service-url> --device-id desktop --token-from-keychain
mw sync doctor --endpoint https://<service-url> --device-id desktop --token-from-keychain
```

Validate token/device mapping explicitly:

```bash
mw sync token check \
  --endpoint https://<service-url> \
  --device-id desktop \
  --token-from-keychain
```

Fetch token dynamically at runtime with `--token-command` (no token stored in shell history):

```bash
mw sync doctor \
  --endpoint https://<service-url> \
  --token-command "gcloud secrets versions access latest --secret hive-sync-api-device-tokens --project <project-id> | tr ',' '\\n' | awk -F= '$1==\"desktop\"{print $2}'"
```

## 4) Retry and failure policy (current behavior)

- Transient failures (`429` / `5xx` / network) trigger exponential backoff retry.
- Non-transient failures fail fast.
- Outbox failures increment attempt counts and preserve pending status for next cycle.

## 5) Conflict handling

- Server push conflict events are persisted to local `sync_conflicts`.
- Conflict artifacts are written to:
  - default: `~/.local/share/mw/conflicts`
  - or `--conflicts-dir`

### Periodic stale-conflict review

Review unresolved conflicts older than 7 days:

```bash
mw sync conflicts review
```

Export the current stale set to JSON:

```bash
mw sync conflicts review --export-dir ~/.local/share/mw/conflicts/reviews
```

After export/triage, mark the exported set resolved:

```bash
mw sync conflicts review \
  --export-dir ~/.local/share/mw/conflicts/reviews \
  --mark-resolved
```

Notes:

- `--older-than` defaults to `168h` (7 days)
- `--include-resolved` widens the report without affecting default hygiene workflow
- `--mark-resolved` requires an export destination for safer review bookkeeping

## 6) Version/base-version behavior

- Local entity versions are tracked in `sync_entity_versions`.
- New local outbox entries use the current local entity version as `base_version`.
- Pulled, applied operations increment local entity version counters.

## 7) Todo sync parity behavior

- Pulled `todo` operations are applied into local `sync_todos`.
- This provides convergence for canonical todo entities independent of note-derived task projections.
- Stored todo shape is task-index aligned (`todo_section`, checkbox state, task text, source note metadata).

## 7a) Current phone-to-desktop visibility expectation

- The Phase 2 PWA pushes note/todo operations into the shared sync stream.
- Desktop `mw sync` can pull and apply those operations into local sync persistence.
- At the moment, this does **not** guarantee visible markdown file changes in the main notes workspace.
- For now, treat successful cursor advancement and applied-operation counts as stronger confirmation than
  filesystem diffs when validating phone-originated sync.

## 8) Troubleshooting quick checks

1. API health:
   - `curl http://127.0.0.1:8080/healthz`
2. API sync state:
   - `curl -H "Authorization: Bearer <token>" http://127.0.0.1:8080/v1/sync/state`
3. Local test suite:
   - `go test ./internal/features/syncclient ./internal/features/syncapi ./internal/infra/db`
4. Full repo tests:
   - `go test ./...`

### Common auth gotcha

If your secret value is formatted like:

```text
desktop=abc123,phone=def456
```

then the actual bearer token for `desktop` is just `abc123` (not `desktop=abc123`).

## 9) Deferred hardening (tracked in project future file)

- Device token rotation workflow.
- Secret Manager-backed device token loading.

Manual token rotation checklist:

- `docs/hive-sync-token-rotation-checklist.md`

## 10) Cloud deployment playbook

For Cloud Run + Cloud SQL deployment steps and scripts, see:

- `docs/hive-sync-cloud-deploy.md`
- `scripts/cloud/bootstrap-hive-sync-infra.sh`
- `scripts/cloud/deploy-hive-sync-api.sh`

## 11) Monitoring and backups

For operational guardrails and data recovery:

- Monitoring + alerts:
  - `docs/hive-sync-monitoring.md`
  - `scripts/cloud/setup-hive-sync-monitoring.sh`
- Backup + recovery:
  - `docs/hive-sync-backup-recovery.md`
  - `scripts/cloud/setup-hive-sync-backups.sh`
  - `scripts/cloud/export-hive-sync-backup.sh`
