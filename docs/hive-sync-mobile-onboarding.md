# Hive Sync Mobile Onboarding

This document covers mobile setup, install checks, and current limitations for the Hive Mind PWA.

## Purpose

Use this guide to:

- configure the mobile PWA against `hive-sync-api`
- validate the current mobile install/sync baseline on a phone
- understand what remains deferred to a later phase

## Prerequisites

- Deployed or locally reachable `hive-sync-api`
- Valid device bearer token
- Device ID mapped to that token
- PWA built and served from `apps/hive-pwa`
- Browser access allowed by `HIVE_SYNC_CORS_ALLOWED_ORIGINS`

## Quick recovery checklist

For this repo's current cloud deployment, use:

- project: `hive-mind-492419`
- region: `us-east1`
- service: `hive-sync-api`
- default Cloud Run URL: `https://hive-sync-api-wr23e5lyna-ue.a.run.app`

### 1) Distinguish the two URLs

- **PWA URL**: where the browser loads the Svelte app, usually `http://localhost:5173` or
  `http://<lan-ip>:5173`
- **API endpoint**: the Cloud Run `hive-sync-api` URL entered into the app settings

Do not put the Vite dev-server URL into the app's **Endpoint** field.

### 2) Resolve the current Cloud Run endpoint

```bash
gcloud run services describe hive-sync-api \
  --region us-east1 \
  --project hive-mind-492419 \
  --format='value(status.url)'
```

Use the returned URL as the app endpoint.

### 3) Read the current device token map

```bash
gcloud secrets versions access latest \
  --secret "hive-sync-api-device-tokens" \
  --project "hive-mind-492419"
```

If the output is `work=aaa,personal=bbb,phone=ccc`, then:

- device ID = `phone`
- token value = `ccc`

Use only the token value, not `phone=ccc`.

### 4) Validate the token before using the phone UI

```bash
PHONE_TOKEN="$(gcloud secrets versions access latest \
  --secret hive-sync-api-device-tokens \
  --project hive-mind-492419 | tr ',' '\n' | awk -F= '$1=="phone"{print $2}')"

mw sync token check \
  --endpoint "https://hive-sync-api-wr23e5lyna-ue.a.run.app" \
  --device-id "phone" \
  --token "$PHONE_TOKEN"
```

### 5) Fix CORS for phone/LAN testing

If the phone opens the PWA from `http://<lan-ip>:5173`, that exact origin must be allowed by
the API deployment.

Example for a LAN IP of `192.168.1.205`:

```bash
cd ~/Projects/mind-weaver

PROJECT_ID="hive-mind-492419" \
REGION="us-east1" \
CLOUD_SQL_INSTANCE="hive-sync-pg" \
CORS_ALLOWED_ORIGINS="http://localhost:5173;http://192.168.1.205:5173" \
bash scripts/cloud/deploy-hive-sync-api.sh
```

Important: assigning shell variables on their own does nothing unless they are exported or passed
inline to the deploy command as shown above.

### 6) Start the PWA

```bash
cd ~/Projects/mind-weaver/apps/hive-pwa
npm install
npm run dev
```

Open the printed **Network** URL on the phone, then in the app enter:

- endpoint = Cloud Run URL
- device ID = `phone`
- token = phone token value

## What to expect right now

The PWA is a local-first mobile client. It can only pull notes that have already been published
into `hive-sync-api`, so the desktop outbox must be drained before expecting the phone to show
the whole notes vault.

### Seed the cloud with desktop notes

If the cloud database does not have every desktop note yet, run this from the desktop first:

```bash
cd ~/Projects/mind-weaver

# Enqueue current markdown note snapshots into the local sync outbox.
mw notes sync

# Push all currently pending outbox operations without waiting between batches.
mw sync --until-empty --outbox-limit 250

# Confirm there are no pending local outbox operations left.
mw sync doctor --skip-remote
```

If `mw sync doctor --skip-remote` still reports pending outbox operations, fix the reported sync
error before using the PWA as the anywhere-notes browser.

### Inside the phone PWA

When you save a note or create/update/delete a todo, the app:

1. writes the item into local IndexedDB
2. enqueues a draft sync operation

You should expect to see:

- the new note/todo appear in the phone UI immediately
- `Queued sync ops` increase before sync

When you tap **Manual Sync**, the app:

1. pushes queued phone changes to `hive-sync-api`
2. pulls remote operations by cursor
3. updates local/remote cursor metadata
4. removes accepted queued operations from the local queue

After a healthy sync, you should usually expect:

- `Queued sync ops` to drop
- sync status text to mention pushed/pulled counts
- local and remote cursor values to advance

### On desktop after `mw sync`

If you run desktop sync after a phone sync, the current expectation is:

- sync cursors may advance
- pulled todo operations are applied into local `sync_todos`
- pulled note operations are applied into the local note DB
- your normal markdown files may show **no visible change yet**

This is expected for the current Phase 2 slice. The phone-to-server-to-desktop path proves sync transport
and local persistence, but it does **not yet** provide a file-materialization workflow back into the
canonical markdown workspace.

## Setup

1. run npm run dev in apps/hive-pwa
2. Open the PWA in the mobile browser.
3. Enter:
   - endpoint
   - device ID
   - bearer token
4. Tap **Validate Token**.
5. Tap **Check Sync State**.
6. Optionally create a local note and todo, then tap **Manual Sync**.

## Install smoke test checklist

Status in this repo session: **procedure documented, physical device execution pending**.

### iOS Safari

1. Open the PWA URL in Safari.
2. Use **Share → Add to Home Screen**.
3. Launch from the home screen.
4. Verify the app opens without browser chrome taking over the full experience.

### Android Chrome

1. Open the PWA URL in Chrome.
2. Use **Install app** / **Add to Home screen**.
3. Launch from the home screen.
4. Verify the app opens as a standalone app shell.

### Required verification

- [ ] App launches from home screen.
- [ ] Endpoint and device ID remain populated after relaunch.
- [ ] Token validation succeeds with a known-good token.
- [ ] Remote sync-state check succeeds while online.
- [ ] Online/offline banner changes when connectivity changes.
- [ ] Local note creation works.
- [ ] Local todo creation works.
- [ ] Manual sync pushes/pulls without fatal UI errors.
- [ ] Local cursor and remote cursor update after sync.

### Suggested evidence to capture

- Screenshot of installed app on home screen
- Screenshot of sync health surface after successful remote check
- Screenshot of local note/todo surfaces with queued work
- Short notes for any mobile-browser-specific behavior

## Known limitations

- Token is manually entered; no refresh or secure device-native storage flow yet.
- Manual sync only; no background sync guarantee.
- Offline edits are queued locally, but replay/conflict UX is still minimal.
- Note browsing/editing is tabbed and search-driven, but still intentionally lightweight with no rich markdown preview mode yet.
- Todo editing/filtering is improved but still focused on baseline payload compatibility rather than full task-management depth.
- Conflict visibility exists in the PWA, but deep review/export remains a desktop CLI workflow.
- Connectivity health is browser-derived and should be treated as advisory.
- Desktop `mw sync` may successfully ingest phone-originated note/todo operations without producing
  obvious markdown file changes in the normal notes tree.

## Deferred follow-up

1. Improve mobile authoring UX for notes and todos.
2. Add explicit conflict inbox/review flows in the PWA.
3. Add safer token handling options for mobile clients.
4. Explore background sync/retry behavior where platform support allows it.
5. Add richer sync diagnostics including cursor lag and queue aging.
6. Add higher-confidence smoke/acceptance coverage across iOS and Android devices.
7. Add richer note preview, sorting, and larger-workspace browsing flows once mobile usage patterns are clearer.

## Operator notes

- If remote checks fail, confirm CORS origin allowlist configuration first.
- If the UI says `Failed to fetch`, treat CORS or endpoint confusion as the first suspects.
- If token validation fails, confirm the token maps to the same device ID entered in the app.
- If `gcloud` complains that `PROJECT_ID` is empty, pass the literal `--project` value or set it
  with `gcloud config set project hive-mind-492419`.
- If manual sync stalls, inspect queued operations and server logs before retrying repeatedly.
