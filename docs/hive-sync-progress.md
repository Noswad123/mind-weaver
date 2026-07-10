# Hive Sync Progress and Next Steps

> Status: parked legacy Hive Sync notes. The Rust `mw` CLI no longer supports
> top-level `mw sync`; local notes use `mw notes sync` / `mw seal`. Revisit these
> commands only if Hive Sync returns under a new boundary.

Last updated: 2026-04-05

## What has been implemented

### 1) Local sync primitives in `mw`
- Added local sync tables in `db/schema.sql`:
  - `sync_outbox`
  - `sync_state`
  - `sync_conflicts`
- Added outbox/state DB operations in `internal/infra/db/sync_outbox.go`.
- Added tests in `internal/infra/db/sync_outbox_test.go`.
- Hooked note lifecycle into outbox enqueueing:
  - note upsert -> `note/upsert` operation
  - note delete -> `note/delete` operation

### 2) Conflict safety artifacts (desktop)
- Added conflict event type in `internal/core/syncops/conflicts.go`.
- Added conflict artifact writer:
  - `internal/infra/fs/conflicts/writer.go`
  - `internal/infra/fs/conflicts/writer_test.go`
- Artifact naming follows:
  - `<timestamp>--<device_id>--<entity_type>--<entity_key>.json`

### 3) `hive-sync-api` service scaffolding and persistence
- Added runtime entrypoint: `cmd/hiveSyncAPI/main.go`.
- Added HTTP API surface:
  - `GET /healthz`
  - `GET /v1/sync/state`
  - `POST /v1/sync/push`
  - `GET /v1/sync/pull`
  - `POST /v1/devices/register`
- Added store abstraction and two implementations:
  - `internal/features/syncapi/memory_store.go`
  - `internal/features/syncapi/postgres_store.go`

### 4) Conflict behavior in API push flow
- Added conflict resolution utility (`internal/features/syncapi/conflict_resolution.go`).
- Conflict trigger: incoming `base_version` differs from current entity version.
- Resolution policy (v1):
  - Last-write-wins by `client_updated_at`
  - Tie-break by `op_id`
  - Final tie-break by `device_id`
- Loser operations are stored as non-applied events and excluded from pull streams.
- Conflict events are persisted in Postgres (`sync_conflicts`) and returned in push response.

### 5) `mw` sync client + convergence tests
- Added sync client module (`internal/features/syncclient/client.go`) with:
  - outbox push handling
  - conflict persistence + artifact writing
  - pull apply loop + cursor persistence
- Added end-to-end sync client tests (`internal/features/syncclient/client_test.go`).

### 6) Auth middleware (device bearer)
- Added device token authenticator + middleware in `internal/features/syncapi/auth.go`.
- Protected routes:
  - `GET /v1/sync/state`
  - `POST /v1/sync/push`
  - `GET /v1/sync/pull`
  - `POST /v1/devices/register`
- Added `hive-sync-api` env parsing for token maps in `cmd/hiveSyncAPI/main.go`.

### 7) Version tracking, todo parity, bounded retries, and observability
- Added local sync entity version tracking (`sync_entity_versions`) and `base_version` on outbox rows.
- Added todo apply parity via local `sync_todos` store.
- Todo sync payload/type shape now aligns with task-index semantics:
  - `todo_section` (`Inbox|Next|Waiting`)
  - checkbox state (`done`)
  - task text
  - source note metadata (`source_id`, `source_path`, `task_scope`, `task_area`)
- Added transient retry/backoff and bounded worker loop in sync client.
- Added sync observability outputs (counters, conflict rate, cursor lag).
- Added operational runbook: `docs/hive-sync-runbook.md`.

### 8) Reliability hardening tests (soak + migration safety)
- Added multi-device offline/reconnect soak coverage in sync client tests.
- Added migration safety tests for legacy local DB schemas:
  - verifies `sync_outbox.base_version` migration
  - verifies task-index-aligned `sync_todos` columns are added
  - verifies task-index todo upserts work after migration

### 9) Sync doctor diagnostics command
- Added `mw sync doctor` (`mind-weaver sync doctor`) for quick health inspection.
- Supports text/json output and optional remote cursor check.
- Reports key local health signals:
  - pending/retried outbox counts
  - unresolved conflicts
  - local cursor
  - tracked entity-version and synced-todo row counts

### 10) Cloud deployment assets (Cloud Run + Cloud SQL)
- Added container image build file: `Dockerfile.hive-sync-api`.
- Added cloud bootstrap script:
  - `scripts/cloud/bootstrap-hive-sync-infra.sh`
- Added cloud deploy script:
  - `scripts/cloud/deploy-hive-sync-api.sh`
- Added cloud deployment guide:
  - `docs/hive-sync-cloud-deploy.md`
- Added safer token handling options for CLI clients:
  - `mw sync token store --token-stdin` (macOS Keychain storage)
  - `--token-from-keychain` / `--token-command` for runtime token retrieval
  - `mw sync token check` for explicit token/device_id validation

### 11) Monitoring + backup/recovery baseline
- Added monitoring setup script:
  - `scripts/cloud/setup-hive-sync-monitoring.sh`
  - uptime check + two alert policies (uptime failure, 5xx rate)
- Added backup setup/export scripts:
  - `scripts/cloud/setup-hive-sync-backups.sh`
  - `scripts/cloud/export-hive-sync-backup.sh`
- Added operational docs:
  - `docs/hive-sync-monitoring.md`
  - `docs/hive-sync-backup-recovery.md`
  - `docs/hive-sync-token-rotation-checklist.md`

### 12) Phase 2 mobile scaffold kickoff
- Added initial PWA app scaffold under:
  - `apps/hive-pwa/`
- Included:
  - Svelte + Vite app shell (TypeScript)
  - manifest + service worker registration baseline
  - minimal token/device validation + sync-state check UI

### 13) Browser compatibility for PWA calls (CORS)
- Added CORS support in `hive-sync-api` with:
  - allowed-origin policy from `HIVE_SYNC_CORS_ALLOWED_ORIGINS`
  - `OPTIONS` preflight handling
  - allow headers for `Authorization` and `Content-Type`
- Added server tests for:
  - allowed/disallowed preflight origins
  - preflight on authenticated routes without requiring bearer token

### 14) IndexedDB foundation for the mobile PWA
- Added browser IndexedDB wrapper under `apps/hive-pwa/src/hiveStorage.ts`.
- IndexedDB schema now initializes stores for:
  - persisted config/settings
  - persisted sync metadata
  - draft sync queue
  - cached note metadata placeholder
  - local note records
  - local todo records
  - local sync entity versions
- PWA settings and last observed sync metadata now persist locally across reloads.

### 15) Minimal mobile note/todo surfaces + manual sync
- Expanded `apps/hive-pwa/src/App.svelte` with:
  - local note list/editor
  - local todo capture/toggle/delete surface
  - manual sync action
- Added PWA sync engine under `apps/hive-pwa/src/hiveSyncEngine.ts`.
- Manual sync behavior now:
  - pushes queued local draft operations to `hive-sync-api`
  - pulls remote operations by cursor
  - applies pulled note/todo operations into local IndexedDB state
  - updates local cursor and last successful sync metadata

### 16) Sync health surface for the PWA
- Added online/offline detection in the PWA via browser connectivity events.
- Added dedicated remote sync-state status surface showing:
  - latest remote check result
  - local applied cursor
  - observed remote cursor
  - observed server time
  - last remote check time
  - last successful sync time
- Remote sync-state result metadata now persists in IndexedDB alongside other sync metadata.

### 17) Mobile onboarding + wrap-up documentation
- Added `docs/hive-sync-mobile-onboarding.md`.
- Documented:
  - setup flow for endpoint/device/token
  - install smoke-test procedure for iOS and Android
  - required verification checklist
  - evidence to capture during manual validation
  - known limitations for the current Phase 2 slice
  - deferred Phase 3 backlog items
- Actual physical-device execution remains a manual follow-up outside this repo session.

### 18) Clarified phone setup + cloud endpoint defaults
- Updated mobile/cloud docs to better distinguish:
  - PWA dev-server URL vs API endpoint URL
  - device-token map entries vs token values
  - shell variable assignment vs inline deploy env overrides
- Added explicit recovery steps for:
  - resolving the current Cloud Run URL
  - extracting a phone token from Secret Manager
  - redeploying Cloud Run with LAN CORS origins for phone testing
- Updated `apps/hive-pwa` so the default endpoint now falls back to the current Cloud Run URL.

### 19) Documented current phone-to-desktop sync expectation
- Clarified that successful phone-originated sync currently proves:
  - local phone persistence
  - queue push into `hive-sync-api`
  - desktop-side local sync ingestion
- Clarified that users should **not yet** expect obvious markdown file changes after desktop `mw sync`
  when validating phone-originated note/todo operations.

### 20) Stale conflict review workflow
- Added `mw sync conflicts review` CLI workflow for periodic conflict hygiene.
- Supports:
  - listing stale conflicts older than a configurable threshold
  - exporting a JSON review snapshot
  - marking reviewed conflicts resolved after successful export
- Added DB-side filtering/resolution helpers for `sync_conflicts` rows.

### 21) Phase 3A synced-content visibility baseline
- Added Phase 3 planning docs under the local project planning directory.
- Updated `apps/hive-pwa` notes/todos surfaces to show explicit:
  - source labels (`local` vs `remote`)
  - sync-state labels (`local-only`, `queued`, `synced`, `conflict`)
- Added synced-content counters in the PWA health/status area.
- Synced entities now retain `lastSyncedAt` metadata locally for clearer trust signals.
- Manual sync now updates local item state after accepted pushes and remote pulls.

### 22) Phase 3B mobile conflict visibility baseline
- Added local IndexedDB-backed mobile conflict log entries for push conflict results.
- Updated the PWA to show:
  - open conflict banner/status
  - recent conflict list
  - winner/loser device metadata when available
  - local mark-reviewed action for mobile triage bookkeeping
- Added guidance in the mobile UI to escalate deeper review/export workflows to desktop CLI.

### 23) Phase 3C notes/todos usability uplift baseline
- Updated note browsing with:
  - search/filter by title/path/content/state
  - selected-note detail summary
  - clearer note stats for mobile editing context
- Updated todo UX with:
  - search/filter by text/state/section
  - selected-todo detail editor
  - section/done/meta editing
  - clearer list/detail split for mobile and desktop-sized layouts
- Kept sync-state trust signals visible while reducing authoring friction.

## Current status summary

- ✅ Local queue and cursor primitives exist.
- ✅ Conflict archive writer exists.
- ✅ Sync API skeleton is running.
- ✅ Push/pull cursor behavior exists.
- ✅ Conflict detection + event payloads exist.
- ✅ Auth middleware + device token enforcement exists.
- ✅ Client conflict artifact wiring exists.
- ✅ Local `base_version` tracking + push propagation exists.
- ✅ Todo pull/apply parity exists (via `sync_todos`).
- ✅ Bounded retry/backoff worker mode exists.
- ✅ Sync observability logs + runbook exist.
- ✅ Multi-device soak + offline/reconnect convergence tests exist.
- ✅ Legacy DB migration safety tests exist for sync tables.
- ✅ `mw sync doctor` command exists for local diagnostics.
- ✅ Cloud Run + Cloud SQL deployment scripts/playbook exist.
- ✅ Monitoring alert baseline exists (uptime + 5xx policy setup).
- ✅ Backup/recovery baseline exists (automated backups + export workflow).
- ✅ Phase 2 PWA scaffold exists in `apps/hive-pwa`.
- ✅ CORS/preflight support exists for browser/PWA clients.
- ✅ IndexedDB schema + local persistence baseline exists in `apps/hive-pwa`.
- ✅ Minimal mobile notes/todos read/write surfaces and manual sync action exist in `apps/hive-pwa`.
- ✅ Sync health surface exists in `apps/hive-pwa`.
- ✅ Mobile onboarding docs, known limitations, and next-phase backlog are documented.
- ✅ Periodic stale-conflict review/export workflow exists via CLI.
- ✅ Phase 3A synced-content visibility baseline exists in `apps/hive-pwa`.
- ✅ Phase 3B mobile conflict visibility baseline exists in `apps/hive-pwa`.
- ✅ Phase 3C notes/todos usability uplift baseline exists in `apps/hive-pwa`.

## Next implementation steps

1. Execute the documented phone smoke tests on iOS/Android and capture results.
2. Add launchd templates (deferred) when scheduler behavior stabilizes across devices.
3. Reassess Phase 4 operational hardening priorities after real mobile usage feedback.

## Verification commands used

- `go test ./internal/features/syncapi`
- `go test ./internal/infra/db`
- `go test ./internal/infra/fs/conflicts`
- `go test ./cmd/mw ./internal/infra/db`
- `npm run check` (from `apps/hive-pwa`)
- `npm run build` (from `apps/hive-pwa`)
- `go test ./...`
- `go run ./cmd/hiveSyncAPI`
- `go run ./cmd/mw notes -help`
