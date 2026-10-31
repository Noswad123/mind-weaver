# Todo workflow roadmap

Status — 2026-10-30: **local cutover DONE**. The explicitly authorized release CLI
installation, editor upgrade, and reviewed real-source migration succeeded. Twelve
sources plus the dashboard migrated; a fresh default plan proposes zero changes
with no blockers. The installed CLI is no longer a gate. Hive coordination, Google
Tasks/Calendar routing, Android delivery, native widgets, and the default life-log
ledger/review-reflection experience remain pending. Resume the Hive Android/Google
vertical slice next, not another local cutover request. Global notes validation still
has a pre-existing duplicate-ID failure; see the cutover record below.

## Overview

Make the personal todo workflow available wherever tasks arise and visible enough
to remember to use. Faster desktop commands alone do not solve the main problem.

The complete loop is **capture anywhere → triage → plan → do → complete → move to
life log → reflect**. Checking a box is an intermediate state, not the destination.
Logging need not be immediate: recommend separate completion and an explicit daily
review action to move selected completed work into the life log. No automatic
archive currently runs.

## Requested changes

- Replace the `task-index` domain requirement with the confirmed **`task`** domain.
  Keep `task_active: true` required; domain-only activation is deferred.
- Keep the parent `## Todo` section, with these child sections in this order:
  **Do Now**, **Inbox**, **Blocked**, replacing Next/Inbox/Waiting vocabulary.
- The reviewed real-notes migration is complete. Do not retain legacy headings as
  ongoing aliases or require the user to maintain both structures.
- Reduce friction when quickly capturing todos.
- Reduce friction when marking todos done.
- Move completed todos into the life log, where accomplishments can be reviewed
  and reflected on. This is core functionality, not optional archive cleanup.
- Support phone capture and integration with Google Calendar. Hive Mind was
  intended to provide this, but the end-to-end workflow is unfinished.
- The phone is **Android**. Tasks with a date should block Google Calendar;
  tasks without a date should populate Google Tasks. This is an automatic routing
  rule, not a requirement to choose an export destination for every task.
- Address remembering to use the system, not just the mechanics of using it.

Work tasks already live in Jira. Recommend leaving Jira authoritative for work;
do not make duplicate entry or a Jira integration a prerequisite for this personal
workflow. Any future work-summary import needs separate scope and privacy choices.

These are user requirements. Below, checked items describe delivered local code;
unchecked items remain pending. Recommendations are not additional requirements.

### Confirmed Google routing and remaining timing choice

| Task scheduling information | Destination |
| --- | --- |
| No date | Google Tasks |
| Date and time, with duration/end or an agreed default | Google Calendar busy block |
| Date only | Google Calendar; representation needs a timing policy |

Date-only tasks must not fall back to Google Tasks merely because a start time is
missing. Choose whether these become all-day events or timed blocks using a
configured time/duration. An all-day entry is not a useful timed work reservation;
do not silently mark an entire day busy or invent an arbitrary appointment.

Workflow section and scheduling are independent: Inbox/Do Now/Blocked express
readiness, while the presence of a date controls the Google destination. Any
exclusions, such as suppressing blocked tasks, require an explicit separate decision.

## Recommended everyday model

| State | Meaning | Typical action |
| --- | --- | --- |
| Inbox | Captured, not yet committed to doing | Clarify, delete, or promote |
| Do Now | Ready and deliberately selected for current attention | Work, complete, or defer |
| Blocked | Cannot proceed until a dependency is resolved | Record what/who is needed and review |

Completion remains a checkbox state (`[x]`), not a fourth workflow section. A
completed task can remain checked until daily review moves it into the life log
with enough context for reflection; immediate archival is not required.
Keep Do Now small (suggested: 3–5 tasks); the inbox is not today's commitment.
For blocked tasks, record the dependency in ordinary child prose; do not require
another metadata format just to capture it.

Canonical new-note shape (other note metadata omitted):

```markdown
---
id: "project-example"
domains: [task]
task_active: true
task_scope: project
---

## Todo
### Do Now
- [ ] Ship the smallest useful improvement

### Inbox
- [ ] Investigate the next idea

### Blocked
- [ ] Publish the release
  Waiting for the review to finish.
```

### Naming and eligibility

- **`task` is confirmed** as the canonical domain, consistent with the existing
  `task_*` metadata. The user-facing CLI remains `mw todos`; `todo` is not an alias.
- Convert `Next` to Do Now and `Waiting` to Blocked during migration, not during
  ordinary runtime reads. The post-migration contract has only the new headings.
- `task_active: true` is still required. Domain-only activation is an optional
  simplification deferred to separate work, not current behavior. Migration preserves
  inactive status rather than enabling previously inactive notes.
- Do not require priority, effort, due date, or area to capture a task. Defaults
  and later triage should handle these; preserve existing metadata when editing.

## Current implementation and friction

This assessment includes the installed Rust CLI and upgraded local Neovim session,
not assumed parity with the legacy Go CLI or remote companions.

1. **The local contract is canonical.** Scanning, task-specific YAML reads, embedded
   schema discovery/validation, formatting, and templates use `task` with
   Do Now/Inbox/Blocked. Active legacy domains/headings fail with migration guidance;
   no ordinary runtime aliases remain. Real notes have migrated; Hive companion
   clients still need a coordinated cutover.
2. **The dashboard only projects Do Now tasks.** Inbox and Blocked remain available
   through source-backed reads; they are not dashboard commitments.
3. **Fast, safe reads already exist.** `mw todos snapshot --json` reads saved
   Markdown without opening the database and provides task IDs, revisions, and
   source provenance suitable for an editor or terminal picker.
4. **Safe editor completion is integrated.** Toggle/update require an observed
   task ID and source revision. A picker can carry those values so the user never
    has to copy them. Explicit desired-state done/reopen is now implemented in the
     CLI and Neovim observed-revision pickers; TUI capture/completion remains pending.
5. **Configured capture now exists.** `mw todos add` writes to configured
   `inbox_path` / `INBOX_PATH` (with `MW_INBOX_PATH` precedence) under the notes root,
   scaffolding a missing/zero-byte source. Parents must already exist. Existing
   sources need an active canonical scaffold with all three unique sections.
   Stable capture IDs provide intentional retry safety, not title deduplication.
6. **Dashboard edits have deliberate limits.** Direct mutations reject pending
   dashboard edits; dashboard checkbox writeback currently accepts changes to at
    most one source file per sync. Do not build a fast completion flow that relies
    on accumulating dashboard edits across many projects.
7. **Checked archive already exists, but is not a reflection experience.** It
   plans and commits moves of selected completed tasks from one source into an
   integrity-checked managed Markdown ledger, with inspection/recovery support.
   It does not automatically choose the life-log destination or offer an editable
   daily/weekly reflection view. Bare `mw todos archive` is not supported.
8. **The active Hive companion has a substantial foundation.** Its PWA creates
   todos, stores them in IndexedDB, and queues manual sync. The Rust desktop bridge
   applies pulled todos to `sync_todos`; notes are staged and can be exported as
   generated review files. Neither establishes automatic todo delivery into the
   canonical Markdown Inbox. The PWA still uses Inbox/Next/Waiting.
9. **Phone reliability and Calendar are unfinished.** Local entity and queue
   writes are separate in the PWA; offline cold launch needs validation/hardening.
   Hive has memory/SQLite server stores, but a durable hosted deployment and
   unattended desktop operation must be established. No Google Calendar adapter
   or OAuth implementation was found in the inspected MindWeaver or Hive code.

Historical Hive findings above are based on prior source and roadmap inspection of
the separate local `/Users/jdawson/Projects/hive-mind` project, not a verified
 deployment or phone test.

## Delivery plan

### 1. Migrate the contract, rather than maintaining two workflows

- [x] Confirm `task` and retain explicit `task_active: true` activation.
- [x] Deliver pure-read migration planning and strict reviewed-plan commit with
      backups, preserving inactive status, identity markers, checkbox states,
      metadata, child prose, comments, fences, and newline style. Complex YAML or
      ambiguous headings block rather than being guessed. No implicit ID assignment.
- [x] Preview and execute the **real notes** migration in a controlled cutover.
      Review all replacements/blockers and refresh observed IDs/revisions afterward;
      positional IDs can change during structural edits.
- [x] Preflight dashboard edits and incomplete archive operations before local cutover.
      Resolve pending checkbox writeback with a compatible old engine **before
      cutover**. Invalid/legacy dashboards block migration; do not silently discard
      or delete them. Use an upgrade window to stop writers, recheck source revisions,
      migrate, validate, regenerate the dashboard, and then resume upgraded clients.
- [x] Update local scanning, task YAML reads, schemas, formatting, templates,
      projection/counts, and this CLI guidance to the canonical contract. Preserve
      dashboard area grouping; only Do Now projects.
- [x] Upgrade and verify the installed local CLI and Neovim integration during the
      cutover; remote Hive clients remain separately pending below.
- [ ] Coordinate Hive PWA types, normalization, stored entities, and queued
      operations. Migrate offline payloads explicitly; old clients must not silently
      turn Do Now/Blocked back into Inbox or reintroduce retired headings.
- [x] Flag legacy active-source domains/headings with an actionable
      migration error rather than accepting them or silently omitting their tasks.
      Legacy spellings are limited to migration and receipt-bound historical archive
      recovery, not runtime aliases. Frozen V1 history retains its original bytes
      and context; managed ledgers are not active workflow sources.

Acceptance: all migrated tasks remain accounted for, inactive notes stay inactive,
only Do Now/Inbox/Blocked are operational, and outdated clients cannot corrupt state.

### 2. Establish a shared capture-to-life-log engine

- [x] Deliver one-prompt Rust CLI capture into configured `Todo → Inbox`, without
      mandatory metadata questions or project selection; scaffold missing/empty
      sources and discover the resulting canonical task without a DB sync.
- [x] Give captures stable identity and same-ID/literal-title/destination retry
      safety. Managed history under the notes root blocks archived IDs; external
      ledgers are not inspected, and deleting source/history loses the receipt.
- [x] Connect Neovim capture to the shared engine, with one prompt and no metadata.
- [ ] Connect phone surfaces to this same source-writing operation,
      not independent Markdown writers. A database row alone is not capture proof.
- [x] Use snapshot-backed Neovim Done/Reopen pickers carrying observed IDs/revisions;
      no manual ID copying or notes-ingestion pipeline for ordinary capture/completion.
- [ ] Add source-aware workflow moves that preserve entire task blocks.
- [x] Deliver checked, desired-state CLI done/reopen; repeating the desired state
      after observing the new revision does not toggle it back or rewrite sources.
      Completion remains distinct from archival.
- [ ] Define the configured life-log destination and record actual completion time
      separately from archive time. Preserve source context and stable task identity;
      do not invent completion dates for historical checked tasks.
- [ ] Provide a separate daily-review "log selected completed tasks" action using
      checked archive planning, commit, and recovery underneath. Keep completion
      distinct from archival; no immediate or automatic archive is required.
      A completed-but-not-yet-logged task remains available until archival succeeds.
- [ ] Hide per-source archive planning behind the UI. Existing archive operations
      are single-source: process distinct plans with truthful partial results,
      rather than promising an atomic cross-project move.
- [ ] Make accomplishments readable in daily/weekly life-log views, with links and
      optional reflection prompts. Keep editable reflections separate from the
      integrity-managed ledger; do not encourage hand-editing its payloads/receipts.

Acceptance: a captured task can be completed, durably moved out of active notes,
and found in a useful reflection view without manual plumbing or lost context.
An interrupted move remains recoverable and never silently loses the accomplishment.

### 3. Finish the narrow Android-to-notes path through Hive

This is the first cross-device milestone, not a general-purpose sync rebuild.
Build on the existing PWA and transport; do not wait for every Hive roadmap feature.

Android recommendation: favor existing Google Tasks/Calendar apps and home-screen
widgets for everyday visibility. Evaluate Google Tasks as the primary quick-capture
surface, with narrow inbound capture/completion support in Hive, rather than making
remembering to open a custom app a prerequisite. The Hive PWA can remain the richer
review/scheduling surface and supply times the Tasks API cannot read. This is a
proposed entry-point choice, not a confirmed requirement to replace the PWA.

- [ ] Make phone capture a home-screen action with one text submission. Save the
      task and outbox record atomically; validate offline cold launch and persistence
      on the actual Android device/browser for the PWA path. If Google Tasks is the
      capture surface, validate its offline-to-Hive delivery separately; do not claim
      control over its internal queue. Add share-sheet/voice entry only if useful
      and supported, after the basic path works. No new native Android app is needed
      for the first delivery.
- [ ] Use stable capture IDs and retry-safe delivery through authenticated, durable
      Hive storage. Verify restart survival and backup/restore; an ephemeral Cloud
      Run filesystem is not durable storage just because it contains SQLite.
- [ ] Run the existing desktop sync worker unattended and bridge received captures
      into the canonical Markdown Inbox using the shared local engine. Start with
      append-only capture, not concurrent remote editing of whole notes.
- [ ] Show distinct states: saved on phone, received by Hive, written to notes,
      and failed/action needed. Retry on supported foreground/reconnection paths;
      do not depend on mobile browsers reliably running background jobs.
- [ ] Publish a minimal source-backed mobile review of Inbox, Do Now, and recent
      accomplishments, with last-sync time. Phone capture must lead back to usable
      review, not a second disconnected task list. Cached views clearly show when
      the desktop last supplied authoritative state.
- [ ] Make desktop unavailability visible. Phone capture remains durable while the
      computer is off; notes materialize when the bridge next runs. If immediate
      remote completion is required, separately design its checked command path.

Acceptance: with the computer off and phone offline, capture a task, reopen the
app, reconnect, then start the desktop bridge. Exactly one task reaches the
canonical Inbox, and the phone accurately distinguishes queueing from delivery.
The phone review reflects the resulting source state and exposes its freshness.

### 4. Route dated tasks to Calendar and undated tasks to Google Tasks

- [ ] Start with one recurring review event linking to the usable review surface.
      This is an adoption aid and can be created manually before API integration.
- [ ] Automatically publish open undated tasks to one dedicated Google Tasks list
      and dated tasks to one selected Google Calendar. Do not require a separate
      scheduling/export action once the task's date has been supplied.
- [ ] Define which source field supplies the routing date (`due`, `start`, or a new
      scheduled date); distinguish deadlines from planned work. Add explicit timing
      metadata for true time blocks. Do not silently reinterpret existing dates
      during migration. Agree the date-only policy before enabling automatic routing.
- [ ] Use the Calendar API for actual busy blocks. The Google Tasks API records
      dates but cannot read/write scheduled time-of-day; it cannot implement timed
      blocking by itself. Avoid trying to model all-day busy as the default workaround.
- [ ] Add Google OAuth for both APIs, minimum scopes, secure refresh-token storage,
      timezone/DST handling, and durable mappings from stable MindWeaver IDs to the
      Google task/list or event/calendar IDs. Reconcile ambiguous create responses
      before retrying; Google Tasks insertion must not be assumed idempotent.
      Adopt an incoming phone-created Google Task's existing ID rather than
      exporting a duplicate back into the same list.
- [ ] Handle date changes as recoverable destination transitions: adding a date
      replaces the managed Google Tasks representation with a Calendar event;
      removing it does the reverse. Confirm destination creation before retiring
      the old representation. Temporary overlap is reported and reconciled, not
      treated as two tasks. Only touch managed items, never unrelated calendar data.
- [ ] Support completion from the managed Google Tasks list so the phone checkbox
      feeds the same checked source-completion and life-log path. Use stable identity
      and fresh source observations; do not bypass revision checks or overwrite
      concurrent edits. Persist the pending request while the desktop is unavailable.
      Retiring a Tasks item during rerouting is not a user completion.
- [ ] Define rescheduling, deletion, reopen, and completed-event behavior. A Calendar
      event ending/deletion is not proof of task completion. Dated events should link
      to a usable finish-and-log action; a deleted Tasks item is not automatically
      an accomplishment either. Resolve conflicting edits explicitly.
- [ ] Keep Markdown authoritative for task ownership and the life log; treat Google
      changes as supported inputs, not arbitrary whole-note replacements. Do not
      resurrect logged tasks on later provider sync. Report Google delivery and
      source materialization separately, including failures and stale state. Hive
      should project an accepted phone capture without waiting for an offline desktop.

Acceptance: undated capture yields one managed Google Task; dated capture yields one
Calendar entry under the agreed timing policy. Adding/removing the date converges to
one active Google representation. Phone completion ultimately reaches the life log.
Retries/outages neither duplicate nor lose tasks, falsely record completion, or
recreate logged work. Review reminders work on Android.

### 5. Validate adoption and reflection in normal life

- [ ] Choose one existing daily cue (for example, checking the calendar) for a
      brief review of Inbox and Do Now, followed by an explicit action to log selected
      completed tasks. Decide on the time with the user rather
      than imposing another schedule to remember.
- [ ] Use one weekly review to revisit Blocked tasks and reflect on the life log:
      what was accomplished, what mattered, and what should change next week.
- [ ] Surface accomplishments in the review flow, not in a hidden archive command.
      Reflection writing is optional; retaining completed work is not.
- [ ] Trial the smallest complete loop for a week before adding broader sync,
      dashboards, analytics, or more mandatory task fields.

Success is fewer lost captures, trustworthy delivery, tasks resurfacing when useful,
and visible accomplishments—not the number of commands or integrations shipped.

## Safety and validation

- Preserve revision checks, locking, dashboard preflight, byte/permission safety,
  and partial-failure reporting. Never silently discard dashboard or unsaved editor
  changes. Stale observations require a fresh user decision, not a blind retry.
- Capture now validates insertion and survivor identity/context explicitly. Future
  workflow moves need their own whole-block preservation proof; do not reuse
  same-task-count mutation reconstruction unchanged.
- Test migration counts/identity and rollback, outdated/offline clients, duplicate
  delivery, local queue failures, offline launch, server restart, delayed desktop
  delivery, Google Tasks/Calendar routing transitions and ambiguous creates,
  date-only/timezone behavior, inbound completion and interrupted life-log recovery.
- Keep source and historical archive wire schemas separate. Reuse checked archive
  safety without rewriting its frozen V1 history. Any new archival fields require
  an explicit versioning decision.
- Measure capture effort and successful source delivery, not just server acceptance.
  Verify logged tasks survive re-reading, disappear from active views, and remain
  accessible through reflection views. Start with the DB-free snapshot; optimize
  scanning only if measured latency warrants it.

## API / Interfaces

Delivered local interfaces and exact safety contracts are documented in the
[CLI reference](cli.md#local-task-workflow):

- `mw todos add "text" [--capture-id <32 lowercase hex>] [--json]` returns the saved
  `task`, `alreadyCaptured`, and `projectionUpdated` with `schemaVersion: "1"`.
- `mw todos migrate plan [--json]` is pure-read JSON: observations, replacements,
  and blockers. `mw todos migrate commit --plan -` accepts strict stdin JSON up to
  32 MiB, rederives the exact reviewed plan, and refuses blockers/stale changes.
  Optional `--bootstrap-legacy-dashboard` validates each unversioned dashboard row
  against source identity/text/area/Do Now/completion before offering an upgrade;
  the reviewed flag and exact dashboard revision are bound into the plan.
- `mw todos done --id ID --expected-revision HASH [--json]`
  and analogous `reopen`. Desired-state idempotence follows revision/dashboard
  checks, not a bypass of them; no automatic archive.
- Existing checked archive plan/commit/inspect/recover remains supported;
  bare `mw todos archive` is not a valid command.

## Data Flow

The delivered local slice is configured Inbox → guarded saved-Markdown insertion →
Do Now dashboard projection, without DB sync. Migration is reviewed JSON → exact
revalidation → backups in `.mw/task-migration-*` → guarded per-source writes →
projection last. It is not multi-file atomic; partial errors include modified paths
and backup location, with no automatic rollback. Inspect before deliberately retrying.

The planned remote path is Android/Google/Hive → durable delivery → the same local
capture operation. It is not implemented by local capture readiness. Completion →
explicit review/logging → reflection is likewise distinct from provider routing.

## Edge Cases

Same ID and literal title/destination retries do not duplicate; different IDs may
capture identical titles. Archived IDs are blocked only by retained managed history
under the notes root. Existing noncanonical/inactive Inbox sources, missing parent
directories, pending/invalid dashboard edits, and ambiguous migration inputs block
operations. Resolve dashboard edits before cutover with a compatible engine; do
not silently discard them. Incomplete archive operations must be resolved before
migration. V1 historical recovery remains receipt-bound, not a legacy runtime mode.

## Example usage

```sh
mw todos migrate plan --json > reviewed-task-migration.json
# Review all replacements and resolve blockers before explicit commit.
mw todos migrate commit --plan - < reviewed-task-migration.json
mw todos add "Call dentist" --capture-id 0123456789abcdef0123456789abcdef --json
mw todos snapshot --json
```

These are command examples; the completed migration is recorded below. For daily
review, select checked tasks and use the [checked archive protocol](cli.md#checked-archive)
with an explicit ledger path; do not hand-edit integrity-managed history.

## Next steps and remaining decisions

1. Resume the Hive Android/Google vertical slice: coordinate Hive types/queues and
   offline clients, then prove durable phone capture reaches the canonical Inbox.
   Local CLI/editor/real-notes cutover is complete; do not request it again.
2. Keep generic index cleanup and optional domain-only activation separate and pending.
   Local checked Done/Reopen selection is delivered.
3. Agree a daily review cue and explicit log-selected-completed action; establish
   the actual life-log layout and a separate reflection view. Immediate archival
   is not required and no automatic archive currently exists.
4. Android is confirmed. Which browser will be used, and is the existing Hive PWA
   installed or any Hive service currently running? Prefer a Google Tasks widget for
   capture, the PWA shortcut, or both? Deployment state has not been verified.
5. For dated tasks without a time, use an all-day calendar entry or a configured
   default time/duration? Which metadata field supplies this date? The routing rule
   itself is settled: date → Calendar; no date → Google Tasks.

### Historical: initial real-notes readiness check (read-only, 2026-10-30)

This records the initial state, not the current post-cutover state.

Configuration resolves to the `darkness` notes tree, its
`introspection/inbox.md`, and `dashboard.md`. The migration preview observed 1,494
Markdown files and proposed 11 source changes. No plan was committed and no note
was rewritten. The preview is not an authorization or reusable commit artifact;
rerun it after resolving blockers and stopping writers.

The initial preview was blocked by a legacy dashboard, an unsupported `Backlog`
workflow section, and malformed frontmatter in several notes (duplicate IDs,
invalid YAML, and an unterminated block). Review these explicitly before migration;
do not guess a Backlog mapping or repair unrelated note content automatically.

The life log already has year/month and reflection notes under
`introspection/life-log/y2026`. Those are editable notes, not managed archive ledgers.
Choose a separate managed task ledger alongside them, and keep reflection prose
outside that ledger rather than overwriting an existing monthly note. Destination
selection and reflection-view integration remain pending.

Local validation: `cargo test --workspace` passes all 230 tests, including capture,
explicit completion/reopen, reviewed migration, and manual checked archival round
trips. Formatting and whitespace checks pass. Workspace Clippy completes with
existing repository warnings; its `-D warnings` gate is not clean. No installed
binary, Hive deployment, Google account, or Android device was changed.

### Historical: preparatory repairs and former cutover gate (2026-10-30)

The following observations predate the successful CLI/editor/source cutover below.

Following approval to continue, six notes were minimally repaired. Byte-identical
originals were verified before edits and retained as non-Markdown `.bak` files under
`$NOTES_DIR/.mw/task-cutover-backup-20261030/`, mirroring their source paths:

- Preserve the three registered slug IDs; change only the duplicate descriptive
  `id` keys to `title` in the two philosophy notes and the 2026 goals note.
- Move the sudo note's five prose lines out of YAML, retaining their text and
  indentation in the body.
- Remove trailing whitespace from the seven-and-seven frontmatter closing fence.
- Move the cybersecurity Backlog checkbox into its existing Inbox and remove the
  duplicate empty Next/Backlog headings. No task title or completion changed.

The dashboard's seven rows match unchecked source tasks. With the new explicit
source-checked legacy bootstrap option, a fresh pure-read preview observes the same
1,494 Markdown files, proposes 12 task-source edits, and reports zero data blockers.
The real dashboard and task domains/headings have **not** been migrated, and no
task was archived. The five metadata fixes and one Inbox repair are the only real
note changes so far; no notes/database sync or registry rewrite was run.

The remaining gate is the live integration, not the data:

1. Neovim resolves `/Users/jdawson/.local/bin/mw`, an older installed engine without
   the new capture/migration/snapshot commands. The generic version is insufficient
   to establish compatibility; check capabilities explicitly.
2. `mw_todos.lua` still writes captures directly and scaffolds task-index with
   Inbox/Next/Waiting. Replace that writer with checked engine capture before
   allowing it to operate on migrated notes.
3. Upgrade the installed CLI and editor integration together, coordinate writers,
   recheck dirty buffers, and review a fresh migration plan before committing it.
   Then validate discovery/projection and editor capture/completion end to end.

Read-only RPC checks of the existing Neovim session showed no modified buffers or
busy MindWeaver operation before repairs, and none of the six repaired files were
loaded afterward. This is a point-in-time observation, not a cross-client lock.
No binary installation, editor configuration change, or remote service change has
been performed during this preparation.

### Completed local cutover (2026-10-30)

The user explicitly authorized CLI installation, editor upgrade, and real-source
migration. All succeeded. Installed release:

```sh
cargo install --path crates/mw --root /Users/jdawson/.local --force --locked --target-dir target
```

- Installed `/Users/jdawson/.local/bin/mw` equals the release binary: SHA-256
  `4861da2523e72849778caa328350abe73812fe44ece0850d3495747577037a37`.
  Previous binary: `43501d464ac53c8a29844470fd66939eeefba0a4cf7b198662d3e7529c0d2f83`.
- Runtime backups are under
  `/Users/jdawson/Projects/darkness/.mw/task-runtime-cutover-20261030/`:
  `mw-previous.tar.gz` (entry `mw`) and `nvim-previous.tar.gz` (entries
  `lua/config/usercmds/mw_todos.lua`, `lua/util/mw_ops.lua`, `lua/config/keymaps.lua`).
  `manifest.txt` records the operational handoff.
- The dotfiles source symlink is active. New `util/mw_capture.lua` delegates capture
  to the engine: one prompt, no metadata, probe before save, cryptographic stable ID,
  `--` title terminator, and writer paths pinned through `NOTES_DIR`, `MW_INBOX_PATH`,
  `DASHBOARD_PATH`. Whole-root dirty-buffer consent and safe reloads apply.
  Retired `ops.edit_file` fails closed even through old callbacks.
- `MWTodoDone` / `MWTodoReopen` use observed-revision pickers. Mappings:
  `<leader>nc` capture, `<leader>nx` done, `<leader>nr` review, `<leader>na` archive.
  Successful Done/Reopen only notifies; errors/conflicts show raw output. No auto archive.
- Targeted live reload preserved the shared ops table. The editor operation guard
  was held throughout install/migration; no dirty buffers or conflicts were found,
  and the guard was released. No restart is needed in the current session; future
  starts read source files. Unrelated modules and dirty dotfiles were not reset/reloaded.
- Reviewed migration result: `backupPath` =
  `/Users/jdawson/Projects/darkness/.mw/task-migration-88Yztg`, 13 modified paths
  (12 sources + dashboard), `projectionUpdated: true`. Originals and confirmed
  `plan.json` are retained there. A subsequent default plan has zero changes and
  `blockers: []`.
- Fresh snapshot: 11 tasks, 7 Do Now, 4 Inbox, 0 completed, 0 stable IDs; tasks on
  6 nonempty notes, 12 active task notes. Migration does not assign existing IDs.
  Real review sync passes: 7 tasks, 1,494 Markdown files, 12 active sources.
  No test tasks, completion, or archival were performed in real data.
- Six approved preparatory repairs + 12 migration sources (cybersecurity overlaps)
  + dashboard account for 18 unique changed note files. Git dirt was preserved;
  no commits. Metadata repair backups remain in `.mw/task-cutover-backup-20261030`.

**Checks and limitation:** all 230 Rust workspace tests pass. Neovim suites pass:
capture 40, context 133, safety 54, tasks 84, real-engine integration 11. Integration
uses the actual installed binary with sandbox notes/temp IDs only and checks
capture/done/reopen, dirty-buffer cancellation, config-file drift, and option-shaped
titles. The separate Neovim archive suite also passes all 416 cases. A final live
snapshot resolves the installed binary and confirms the same 11 tasks; the Done
mapping and commands are available, with no busy operation or buffer conflicts.

Read-only `notes validate --all` still fails on the pre-existing duplicate ID
`work-reinvention-pm-enablement` in
`moderna/moderna-it/work-reinvention/work-reinvention-pm-enablement/{hub.md,work-reinvention-pm-enablement.md}`.
It was not auto-fixed: **global notes validation has not passed**. No broad ingestion,
database refresh, or `.mw` registry rewrite ran. The source-backed workflow is
operational, but DB-cached note queries may retain old metadata until deliberate refresh.

**Coordinated rollback only:** do not restore the old binary alone—it ignores the
new domain. Compare current files against stored post-state and backups before
explicitly restoring notes, dashboard, editor, and binary as a coherent set. Preserve
all subsequent user edits. `plan.json` maps `changes[N].sourcePath` to
`source-N.bak` and stores source post-state in `changes[N].replacement`;
`dashboard.bak` is the pre-migration dashboard, not a post-state receipt. Inspect
the current dashboard and later edits separately. Never blindly extract tar backups
over later work. There is no automatic rollback.

Out of scope for the first usable loop: Jira mirroring, arbitrary bidirectional
Calendar editing, multi-user hosting, concurrent whole-note editing, automatic
free-slot selection, and a general-purpose replacement for every Hive feature.
Date-based routing and narrow Google Tasks capture/completion are in scope;
automatic time-of-day selection is not implied by that routing rule.

## Related files

- [Scanning, parsing, and identity](../crates/mw-notes/src/todos.rs)
- [Task-specific YAML reader](../crates/mw-notes/src/todos/source.rs)
- [Mutation and dashboard safety](../crates/mw-notes/src/todos/safety.rs)
- [Domain schemas and validation](../crates/mw-notes/src/domain_validation.rs)
- [Formatting](../crates/mw-notes/src/format.rs)
- [Current embedded task schema](../legacy/go/internal/schema/templates/task.yaml)
- [Configured capture](../crates/mw-notes/src/todos/safety/capture.rs)
- [Checked migration](../crates/mw-notes/src/todos/safety/migration.rs)
- [CLI workflow tests](../crates/mw/tests/todos_workflow.rs)
- [CLI entry points](../crates/mw/src/main.rs)
- [Configuration and inbox defaults](../crates/mw-core/src/config.rs)
- [Checked archival contract](cli.md#checked-archive)
- [Broader enhancement themes](enhancements.md)
- [Google Tasks resource and API date/time limitations](https://developers.google.com/workspace/tasks/reference/rest/v1/tasks)
- [Google Calendar events: timing and busy/free semantics](https://developers.google.com/workspace/calendar/api/v3/reference/events)

Companion references (separate local Hive Mind repository):

- `plan.md` and `docs/product-architecture-roadmap.md`
- `hive-pwa/src/App.svelte`, `hiveStorage.ts`, and `hiveSyncEngine.ts`
- `hive-pwa/public/sw.js`
- `crates/hive-desktop/src/lib.rs` and `crates/hive-server/src/lib.rs`
