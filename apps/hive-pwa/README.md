# Hive PWA

> Status: parked optional Hive app-suite notes. The Rust `mw` CLI no longer
> supports top-level `mw sync`; local notes use `mw notes sync` / `mw seal`.
> Revisit the desktop sync commands below only if Hive Sync returns under a new
> boundary.

This folder contains the mobile PWA for Hive Mind notes/todos sync.

## Current scope

- Svelte + Vite app shell (TypeScript enabled)
- Web manifest + service worker registration baseline
- Minimal sync settings UI (endpoint, device-id, token)
- IndexedDB-backed local note and todo surfaces
- Draft queue persistence for local note/todo changes
- Manual push/pull sync action against `hive-sync-api`
- Remote sync-state check action
- Online/offline + sync health status surface
- Explicit local/remote + local-only/queued/synced/conflict item labels
- Mobile conflict banner/log baseline
- Tabbed mobile navigation with focused Notes, Todos, Sync, Conflicts, and Settings sections
- Note search/detail/edit context and todo filter/edit baseline

## Seed desktop notes before mobile browsing

The PWA pulls notes from `hive-sync-api`; it cannot see desktop markdown files until those notes
have been enqueued and pushed from the desktop.

```bash
mw notes sync
mw sync --until-empty --outbox-limit 250
mw sync doctor --skip-remote
```

Then open the PWA and tap **Sync now** from the top status strip.

## Local development

```bash
cd apps/hive-pwa
npm install
npm run check
npm run dev
```

The app itself will usually be reachable at:

- local browser: `http://localhost:5173`
- phone on same LAN: `http://<your-machine-lan-ip>:5173`

Important: that Vite URL is the **PWA URL**, not the sync API endpoint.

## Build

```bash
npm run build
npm run preview
```

## Environment

Copy `.env.example` to `.env` and set values as needed.

`VITE_HIVE_SYNC_API_URL` is used as the default API endpoint.

If `.env` is omitted, the app currently falls back to the deployed Cloud Run endpoint:

`https://hive-sync-api-wr23e5lyna-ue.a.run.app`

## Important note

Browser access to sync API requires CORS support in `hive-sync-api`.

If the PWA is opened from `http://<lan-ip>:5173`, that exact origin must be present in
`HIVE_SYNC_CORS_ALLOWED_ORIGINS` on the Cloud Run deployment.

For mobile setup, smoke-test steps, and known limitations, see
`/Users/jdawson/Projects/hive-mind/docs/hive-sync-mobile-onboarding.md`.
