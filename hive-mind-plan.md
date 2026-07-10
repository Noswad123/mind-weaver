# Hive Mind Plan

Last updated: 2026-07-10

## Direction

Hive Mind can be treated as its own optional app suite instead of being fully
folded into the Rust `mw` notes CLI.

MindWeaver remains the local-first notes system. Hive Mind provides optional
multi-device sync, cloud API, and mobile/web companion surfaces around that
local notes system.

## Current priority

Hive Mind is intentionally parked for now. This document is the breadcrumb trail
for returning later; do not spend significant implementation or design time on
Hive Sync while the main effort is porting the remaining non-Hive Go features to
Rust.

When returning, resume from the existing Go sync API/client/PWA behavior rather
than redesigning from scratch.

## Current components

### 1. Desktop MindWeaver / `mw`

- Rust `mw` is now the primary local notes CLI.
- Legacy Go `mw` remains buildable under `legacy/go` and is still the full sync
  client oracle.
- Rust `mw` does not expose a top-level `mw sync` command; local notes use
  `mw notes sync` / `mw seal`.
- The Rust database layer still has sync/outbox primitives that can be reused if
  Hive Sync returns under a non-conflicting boundary.
- Full HTTP push/pull sync remains parked with the legacy Go/Hive code until a
  new Hive boundary is chosen.

### 2. `hive-sync-api`

Current Go API service capabilities:

- Entrypoint: `cmd/hiveSyncAPI`.
- HTTP endpoints:
  - `GET /healthz`
  - `GET /v1/sync/state`
  - `POST /v1/sync/push`
  - `GET /v1/sync/pull`
  - `POST /v1/devices/register`
- Storage backends:
  - in-memory store for local/dev usage
  - Postgres store for deployed usage
- Device bearer-token auth middleware.
- CORS/preflight support for browser/PWA clients.
- Conflict behavior:
  - detects stale `base_version`
  - resolves by last-write-wins using `client_updated_at`
  - tie-breaks by `op_id`, then `device_id`
  - persists conflict events
  - excludes losing operations from pull streams
- Cloud deployment support:
  - Cloud Run container file
  - Cloud SQL bootstrap/deploy scripts
  - monitoring setup script
  - backup/export scripts

### 3. Legacy Go desktop sync client

Current full desktop sync client capabilities:

- Pushes local `sync_outbox` operations to `hive-sync-api`.
- Pulls remote operations by cursor.
- Persists local cursor in `sync_state`.
- Tracks entity versions in `sync_entity_versions`.
- Applies pulled note/todo operations into local sync persistence.
- Supports todo sync parity through `sync_todos`.
- Handles transient retry/backoff and bounded worker mode.
- Reports observability counters:
  - pushed accepted/rejected
  - pulled applied
  - conflicts logged
  - conflict artifact count
  - cursor lag
- Supports token input through:
  - explicit `--token`
  - `HIVE_SYNC_TOKEN`
  - `--token-command`
  - macOS Keychain via `--token-from-keychain`
- Supports conflict hygiene:
  - local conflict persistence
  - artifact writing
  - stale conflict review/export/mark-resolved workflow

### 4. `apps/hive-pwa`

Current PWA capabilities:

- Svelte + Vite app shell.
- Web app manifest and service worker registration baseline.
- Endpoint/device/token setup UI.
- Token/device validation against the sync API.
- IndexedDB persistence for:
  - settings/config
  - sync metadata
  - draft sync queue
  - note records
  - todo records
  - local sync entity versions
  - conflict log entries
- Minimal note list/editor.
- Minimal todo capture/toggle/delete/edit surfaces.
- Manual sync action:
  - pushes queued local note/todo operations
  - pulls remote operations by cursor
  - applies pulled operations into IndexedDB state
  - updates local cursor and last-success metadata
- Sync health UI:
  - online/offline status
  - local applied cursor
  - observed remote cursor
  - observed server time
  - last remote check
  - last successful sync
- Sync trust signals:
  - local vs remote source labels
  - local-only / queued / synced / conflict states
  - synced-content counters
- Mobile conflict visibility:
  - open conflict banner
  - recent conflict list
  - winner/loser device metadata
  - local mark-reviewed action
- Notes/todos usability uplift:
  - search/filter
  - selected item detail panes
  - editable todo section/done/meta fields

## Current sync model

- Entity types: `note`, `todo`.
- Operation types: `upsert`, `delete`.
- Desktop/source local changes enqueue outbox rows.
- Clients push operations to the API with idempotency keys and base versions.
- Clients pull operations from the API by cursor.
- Conflict detection is version-based.
- Markdown remains the canonical MindWeaver source of truth on desktop.
- SQLite/IndexedDB are projections and sync caches.

## Known limitations

- Rust `mw` does not yet implement full HTTP push/pull sync.
- Phone-originated operations can sync into desktop-side local sync persistence,
  but should not yet be expected to create obvious Markdown file changes in the
  main notes workspace.
- Physical-device PWA smoke tests still require manual execution and evidence
  capture.
- The PWA is useful as a companion capture/review app, not yet a complete mobile
  replacement for the desktop Markdown workflow.
- Launchd/periodic desktop sync templates are deferred until real usage hardens
  scheduler expectations.

## Product boundary

Treat Hive Mind as an optional app suite with a stable integration contract:

- `mw` owns local Markdown notes, ingestion, querying, todos, projections, and
  the desktop source-of-truth workflow.
- Hive Sync owns remote operation exchange, cursoring, conflict events, device
  auth, and cloud/mobile sync surfaces.
- Hive PWA owns mobile/web capture, review, lightweight editing, sync health, and
  conflict visibility.
- The bridge is the sync operation protocol plus local projection stores.

This means the Rust port does not need to absorb every Hive feature before the
Go sync stack can remain useful. Hive can evolve as a separate app while sharing
the same operation model and storage contracts.

## Recommended next steps

These are deliberately deferred until after the main Rust `mw` local-notes port
is further along:

1. Keep `legacy/go` buildable as the full sync client until Hive is split or
   re-homed cleanly.
2. Decide whether the long-term desktop sync client should be:
   - kept in the Hive app suite, or
   - ported later into a Rust `mw-hive`/`hive` binary.
3. Prioritize validation over porting breadth:
   - run iOS/Android PWA smoke tests
   - verify Cloud Run sync state
   - verify desktop cursor advancement
   - verify conflict visibility and review flow
4. Define the stable sync protocol contract in one place:
   - operation JSON shapes
   - idempotency key rules
   - cursor semantics
   - conflict event shape
   - auth requirements
5. Only after the contract is stable, choose whether to extract Hive into its
   own repository/app packaging boundary.

## Resume checklist for later

Before doing new Hive work, quickly re-check:

1. `legacy/go/internal/features/syncclient/client.go`
2. `legacy/go/internal/features/syncapi/`
3. `legacy/go/internal/infra/db/sync_*.go`
4. `legacy/go/internal/mwcli/buildSync*.go`
5. `apps/hive-pwa/src/hiveSyncEngine.ts`
6. `docs/hive-sync-runbook.md`

Then decide whether the work belongs in:

- existing legacy Go Hive code,
- a new standalone Hive binary/app boundary, or
- the Rust `mw` workspace.

## Reference docs

- `docs/hive-sync-progress.md`
- `docs/hive-sync-runbook.md`
- `docs/hive-sync-cloud-deploy.md`
- `docs/hive-sync-monitoring.md`
- `docs/hive-sync-backup-recovery.md`
- `docs/hive-sync-mobile-onboarding.md`
- `apps/hive-pwa/README.md`
