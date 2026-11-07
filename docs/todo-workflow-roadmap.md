# Todo workflow roadmap

Status — 2026-11-01: **local cutover DONE; new scheduled phone-to-editor-owned writer delivery and repeat nonduplication live-verified**. The explicitly authorized release CLI
installation, editor upgrade, and reviewed real-source migration succeeded. Twelve
sources plus the dashboard migrated; a fresh default plan proposes zero changes
with no blockers. The installed CLI is no longer a gate. Manual Android capture and
scheduled Google Tasks source writing are live-verified. Live computer-off catch-up,
pending-job dirty-buffer delivery proof, Calendar routing, native widgets, and the default life-log ledger/review-reflection
experience remain pending. Continue the Hive Android/Google vertical slice, not
another local cutover request. Global notes validation still
has a pre-existing duplicate-ID failure; see the cutover record below.

Hive follow-up: **desktop-local Google Tasks inbound foundation plus OAuth
sign-in/refresh/storage implemented; first Android-to-Markdown capture live-verified**. User-invoked `hive captures` reads a full paginated list into a private
dedicated queue, then delegates stable-ID Inbox creation to the existing `mw`
engine and retains a canonical receipt. Google holds phone captures while the
desktop is off; there is no Hive ACK then. See the sibling Hive
[capture runbook and activation gates](../../hive-mind/docs/google-tasks-capture.md).
The requested next sign-in/refresh slice is code-complete: Desktop PKCE login,
native non-synchronizing macOS Keychain storage, verified Google issuer/subject
namespaces, and once-per-command refresh. See the sibling
[Google sign-in setup guide](../../hive-mind/docs/google-auth.md). The client JSON was
provided and Git-ignored; browser consent, Keychain persistence, and automatic refresh
now succeed. Tasks list access also succeeds after API enablement. The dedicated
`MindWeaver Inbox` Google list is selected and baselined empty in a private Hive
queue. One new undated Android task has now been delivered into
`introspection/inbox.md` under Todo / Inbox, with one durable writer receipt.
Repeating full collection/delivery produced no second task and no Inbox/dashboard
byte or mtime changes. Neovim's temporary guard was released without conflicts;
the captured task remains open. The stable Hive release is now installed, the
existing queue configured, and Neovim registered with its shared operation guard
preserved. Two controlled cycles and actual launchd startup/interval cycles each
observed the same unchanged task without staging or delivering another copy.
The owned LaunchAgent is enabled at a 60-second interval with a 45-second cycle
budget and delivery limit 1. Live dirty-buffer readiness deferral was verified
without changing any real note. On 2026-11-01 the user added the undated, title-only
phone task `Verify scheduled phone capture`. An automatic scheduled cycle (not
manual `run-once`) completed with observed 2, staged 1, unchanged 1, delivered 1,
pending 0. Its one-attempt durable `todos-add` receipt records `alreadyCaptured: false`,
`done: false`, `projectionUpdated: true`, `sourcePath: introspection/inbox.md`,
Todo / Inbox, line 12; `sourceRevision` matches the actual Inbox hash. Actual
Markdown and recursive whole-notes-root search confirm exactly one unchecked
stable-marker source entry. The same-cycle native Neovim bridge reached `phase: done`,
delivered 1, `writerExited: true`, conflicts 0; afterward `busy: false`, mode `n`,
no dirty buffers. The fresh next actual scheduled cycle observed 2 unchanged,
staged/delivered 0, `no_pending`, `authActionRequired: false`, with the owned service
active (`loaded_owned`). Both captures retain one attempt and their receipts.
Inbox (324 bytes) and dashboard (4,651 bytes) bytes/sizes/mtimes stayed unchanged
from post-delivery to follow-up; dashboard content retained its earlier hash, but
its mtime changed on first delivery only. No Google/source completion or archival
was performed. Live computer-off catch-up and pending-job dirty-buffer delivery
proof remain pending; read-only dirty readiness is already proven.
Google completion, outgoing publication, Calendar routing,
and life-log automation remain pending.
`lists`/`poll`/`run` require exactly one `--connection NAME` or explicit
`--access-token-env HIVE_GOOGLE_ACCESS_TOKEN`. Saved mode forbids `--account-key`;
legacy token-env polling/running requires it. No saved-credential failure falls
back to ambient tokens. Queue V2 preserves IDs/receipts, but forbids verified/legacy
connections for the same list in either direction, including empty legacy baselines.
Explicit legacy-binding migration remains future work; do not bypass the guard
with a new database/namespace and risk duplicate captures.

At the inbound inventory-fix checkpoint, 270 normal Hive workspace tests passed, five ignored by
default; background/auth/capture strict Clippy and both explicitly gated real-Neovim
integrations pass. Real-writer integrations passed earlier; see the Hive runbook
for the exact validation boundary. Existing legacy workspace warnings remain.
Latest installed credential access and scheduled polling now pass; earlier stalls
no longer reproduce, and their cause was not established. The original capture
still has one attempt and its receipt, as does the new scheduled capture; the
scheduled repeat left post-delivery Inbox/dashboard bytes and mtimes unchanged.
This is live proof, not added automated tests; later Calendar fixture validation
is recorded in the sibling release checkpoint linked in section 4. Calendar
publication/export/completion writeback and PWA migration remain
pending. The historical remote assessment
and broader unchecked milestones below are not claims that this narrower local
foundation is absent or that hosted Postgres is required before using it.

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
    was found at that historical inspection; a dormant private HTTP adapter is now
    implemented and loopback-tested as recorded in section 4, not activated.
    Hive's narrower Tasks-readonly + openid
   OAuth implementation is now complete as recorded above; Calendar activation
   remains pending. Narrow live Tasks capture and scheduled editor-owned delivery
   are now verified above.

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
      Narrow Google Tasks milestone is live-verified: new scheduled phone capture
      reached the editor-owned shared writer once, with a duplicate-free scheduled
      repeat. Broader phone/PWA surfaces remain pending.
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
The delivered Google inbound lane does not require hosted transport or Postgres.

- [x] Implement user-invoked Google Tasks inbound queue/durable canonical receipts
      and Desktop OAuth sign-in, refresh, secure macOS storage, and verified identity
      binding. Code completion is not real-account or Android activation.
- [x] Activate with the user's private Desktop client JSON path, explicit consent,
      actual Keychain access, list selection, bootstrap decision, and Android-to-Inbox
      receipt/repeat-run proof. No new local cutover is needed.

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
      Narrow milestone complete: scheduled Google Tasks delivery through the native
      Neovim-owned writer and next-scheduled-cycle nonduplication are live-verified.
      This broader remote/PWA item stays unchecked; live computer-off catch-up and
      pending-job dirty-buffer delivery deferral are still unperformed.
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

Hosted publication architecture accepted on 2026-11-01: a GitHub-hosted Linux
workflow after a successful notes push, reading the published commit and retaining
publication outcomes in a separate Git-backed state branch. No paid database/server
is planned initially; verify included usage and actual overage blocking before
activation. The private notes repository has Actions enabled and uses `main`, but
no publisher workflow is installed. Keep current desktop inbound capture separate.
Hive retains the credential-free source-only preview and now implements built-debug
`./target/debug/hive calendar plan`, using a private hashed copy of the existing
DB-free MW snapshot parser over full pinned source and optional ledger commits.
Planning requires independent source/account/concrete-calendar bindings, plus either
explicit observation-only bootstrap or all three committed-ledger inputs. Bootstrap
does not create authoritative state, assign UUIDs, or enroll; missing/bad existing
state errors, never falls back. Strict V1 ledger mappings support markerless sources;
source HTML markers are optional. Decisions are `noop`, advisory `update`, `review`,
`blocked`, or `retired`, never create/delete. A noop only compares against the last
provider acknowledgement, not current remote existence/state. Pending operations
block retries; retired entries never auto-enroll/resurrect, and completed/undated/
invalid source is reviewed, not deleted.

`publicationReady`, `remoteVerified`, and `ledgerHeadVerified` are always false;
`requiresLedgerCompareAndSwap` is always true. Offline planning cannot establish
the latest ledger head. Built-debug `./target/debug/hive calendar enroll` / `reack`
generate local ledger candidates only; reviewed local persistence is now implemented
separately below. Normal-user UI and integration activation remain future work,
not user-maintained IDs/JSON.
Any edit anywhere in an acknowledged file, including unrelated prose, or a move
requires review/new explicit acknowledgement; different parser builds/platforms
also require review/re-acknowledgement. No title/fuzzy/comment-only matching or
friction-free continuity is claimed. Exact LF/CRLF literal bytes and full-file
hashes are preserved.

The historical plan-only macOS checkpoint records 339 normal workspace tests passed, nine ignored;
Calendar crate 62 normal/two ignored and CLI plan five normal/one ignored. The new
real-MW markerless plan gate passed with LF and CRLF, and both earlier real-MW preview
gates were rerun and passed. Strict Calendar Clippy, formatting, build/help passed;
focused review closed the CRLF P2 and bootstrap acknowledgement-validation gap with
no remaining local P1/P2. These are recorded implementation results, not tests run
for this documentation update, and use isolated fixtures, never live user notes or
ledger. **Linux is untested and explicitly deferred by the user until later before
deployment, not a local-work blocker.** No repeated Docker checks/target installs.
Installed binaries, inbound worker/private queue/auth are untouched; no refs,
commits, pushes, workflows/cloud resources, secrets, or billing changed.
See [built-debug usage and privacy warning](../../hive-mind/README.md#calendar-publication-plan-local-only),
[ledger contract](../../hive-mind/docs/product-architecture-roadmap.md#calendar-ledger-schema-v1),
and [historical plan evidence](../../hive-mind/docs/release.md#calendar-plan-checkpoint-2026-11-02).

**Local candidate-generation milestone DONE, not installed; generation does not persist:** both
commands require all source/binding/ledger flags, full source/base commits, and
`--source-path` relative Markdown / positive one-based `--source-line` selection.
Enroll needs open valid-due source and assigns an OS-entropy UUIDv4 without HTML
markers or note edits. Initialization requires explicit `--initialize-ledger` and
proof of path absence at that base commit, with any existing parent components
trees, not authoritative HEAD or remote absence. Normal missing state never
initializes. Duplicates/reservations include retired/pending evidence. Its global
guard requires all old source bases recognized, **including retired**, and no
pending operations. Missing/moved/edited retired basis can block unrelated new
enrollment until a future resolution workflow. No force override or friction-free
restore is promised.

Reack explicitly maps either `--old-source-path` / `--old-source-line` or advanced
`--logical-id` (never both) to the new source selection. It retains the same UUID
across moves/due changes, provider ACK/lifecycle, and unselected pending/tombstones;
the whole old proof is validated before replacement. Pending/retired targets refuse.
`--accept-parser-change` approves only selected-entry digest migration, not old
semantics. Done/genuinely undated ACKs are allowed, not completion/deletion
propagation; shared due validation refuses malformed/duplicate dates even when done.
There is no automatic fuzzy identity matching.

Fully serialized stdout is `kind: "calendar-ledger-candidate"`, operation enroll or
reack, `sourceCommit`, `parserSha256`, `logicalId`, `base` (commit/path/`gitBlobID`
value or null and `existing_blob_at_commit` / `path_absent_at_commit` condition),
and whole V1 `candidateLedger`. Publication/remote/head flags remain false and CAS
is required. Generation persists nothing; enrollment IDs can differ on rerun until
accepted. Never publish from an unpersisted candidate. The reviewed local writer is
now implemented below; exact base blobs alone remain insufficient HEAD proof.
Actual user-repository apply is not activated or approved by this documentation.

Historical candidate-only macOS implementation results: 363 normal workspace tests passed,
ten ignored (`cargo test --workspace --locked --offline -- --test-threads=1`);
Calendar 82 normal/two ignored; CLI candidates four normal/one ignored. All four
explicit real-MW gates passed (CLI candidate/plan/preview and source-reader fixture).
The new gate covers markerless enroll, moved reack preserving UUID/provider ACK,
done ACK, and invalid due on completed source refusing. Nonignored CLI tests prove
forged old ACKs cannot be washed out and rollback refuses with empty stdout. Strict
Calendar all-targets Clippy, fmt/build passed; review found no local-candidate P1/P2.
These are recorded results, not rerun here; fixture commits were only in temporary
test repositories. No actual notes/ledger/state/provider/auth changes, installed
binary swap, capture worker changes, workflow/deploy/billing/secret changes occurred.
Linux remains deferred before hosted deployment, not blocking local development.
See [candidate commands](../../hive-mind/README.md#calendar-ledger-candidates-local-only)
and [historical candidate evidence](../../hive-mind/docs/release.md#calendar-candidate-checkpoint-2026-11-02).

**Local persistence code/test milestone DONE; integration activation not done:**
built-debug, not installed, `calendar verify-candidate` validates/replays the exact
approved candidate and computes ledger blob/tree/commit OIDs without writing Git
objects or refs. `calendar apply` explicitly writes local Git objects and performs
one old-OID HEAD CAS of `refs/heads/hive-calendar-state`, not a push, remote sync,
or Calendar write. Both require explicit source repo/full commit/trusted MW,
independent source/account/concrete-calendar bindings, absolute `--ledger-repo`,
relative `.json` `--ledger-path`, fixed `--state-ref`, absolute regular JSON
`--candidate`, and independently approved 64-lowercase-hex `--candidate-sha256`.
`--create-ref` is initialization-only; selected reack old-parser changes require
independent renewed `--accept-parser-change`. Pin the parser artifact for recovery.

The candidate is captured once, bounded to 8 MiB, with `O_NOFOLLOW`; unknown,
duplicate, and trailing JSON refuse. SHA-256 approves exact reviewed bytes, not a
cryptographic signature/provider proof or arbitrary replacement authority. Original
ledger/history validation precedes semantic replay and whole-candidate equality.
Approved enrollment UUIDs are never reallocated; persistence uses canonical compact
ledger JSON. Machine-generated proposals and review-first tools are the workflow,
not manual ID maintenance or blind hash-and-auto-write commands.

V1 only accepts a **dedicated bare files-backend state repository with no linked
worktrees**, refusing a normal notes worktree, foreign existing tips, symbolic/dangling
refs, and object-store indirection. Updates require a strict tool-shaped ownership
receipt bound to ref/path/binding/blob (not cryptographic signer proof), matching
old blob, and the fixed ref's **whole-commit HEAD** equal to the candidate base.
Init requires ref absence CAS against zero; path absence at the seed alone is not
branch or remote absence proof. The new commit's sole parent is the base and it
inherits the **whole seed tree, possibly notes**, not an orphan state branch. No
adopt/reset/force/push. Raw-tree Git plumbing preserves unrelated modes/OIDs without
checkout/index/config edits/hooks/signing/maintenance. Fixed identity and UTC parent
committer time + 1 second, not approval wall-clock time, give identical verify/apply
expected commit IDs.

**Operator MUST exclusively administer the state repository during the operation.**
Preflight cannot atomically prevent concurrent worktree/config administration;
CAS handles ledger writers, not admins. Readiness is local persistence under this
bare/exclusive-administration assumption, not general deployment readiness.

Verify statuses are `verified`/`already_applied`; apply statuses are
`applied`/`already_applied`. Only actual successful CAS sets `localRefUpdated: true`;
`publicationReady`, `remoteVerified`, and `remoteRefVerified` remain false. Exact
retry at the expected tip writes nothing; later advancement refuses without rewind
or content-only acceptance. Recovery requires the same approved candidate,
destination, bindings, source commit, parser artifact, and permission flags. Root
cancellation spans capture/replay/writer/CAS, including helper phase gaps. SIGINT/
SIGTERM observed before CAS stop ref advancement. Object writes may leave unreachable
objects on failure, with no automatic GC. After CAS dispatch, cancellation or lost
response/output may mean success: report unknown and retry exactly, never rollback.

Historical macOS persistence results after the root-cancellation fix: **389 normal workspace
tests passed, 12 ignored**, using
`cargo test --workspace --locked --offline -- --test-threads=1`; Calendar **105 normal,
3 ignored**; CLI apply **3 normal, 1 ignored**. The new real-MW apply fixture passed
init candidate -> read-only verify -> local apply -> exact retry -> source move/reack
-> verify/update -> stale retry refusal. Four earlier real-MW gates were rerun and
passed, for **five gated fixture passes**. Root phase-boundary SIGINT/SIGTERM,
pre-CAS cancellation after real object writes, and actual successful CAS followed
by injected cancellation/unknown/exact retry were tested. Strict Calendar all-targets
Clippy, fmt/build passed; focused review closed the lost-cancellation P2. The main
CLI's four original legacy warnings are unchanged. These results were supplied,
not rerun for this docs update; older milestone counts remain historical.

All actual Git writes used temporary bare state repositories, not actual user notes,
source/refs, or a new real state repository. No hosted workflow, Calendar/grant,
credentials/Keychain, provider, billing, remote, installed-binary, or worker changes.
At this historical checkpoint, normal-user UI/integration activation and durable
provider intents/results were pending; offline intents are now delivered below.
Real provider/coordinator wiring (remote APIs and Git are not one transaction;
private local ACK durability is delivered below), hosted CI serialization, pinned Linux
runtime, account hard stop, OAuth grant/executor, and remote lease/CAS push remain
pending. No paid service by default. Linux is explicitly deferred until later before
deployment, not a local blocker; no VM probes or target installs are requested.
See [review-first verify/apply examples](../../hive-mind/README.md#calendar-local-persistence),
[persistence contract](../../hive-mind/docs/product-architecture-roadmap.md#calendar-local-approval-and-persistence),
and [historical persistence evidence](../../hive-mind/docs/release.md#calendar-local-persistence-checkpoint-2026-11-02).

**Offline Calendar operation stage DONE, not an actual publisher:** built-debug,
not installed, `calendar prepare` proposes a persistable V2 frozen create/update
intent; `calendar mark-uncertain` proposes a monotonic frozen recovery candidate;
`calendar simulate-ack` emits only an operator-asserted **nonpersistable hypothetical
result**. See [flags/examples](../../hive-mind/README.md#calendar-offline-operations),
[schema/encoding contract](../../hive-mind/docs/product-architecture-roadmap.md#calendar-offline-operation-contract),
and [historical offline macOS evidence](../../hive-mind/docs/release.md#calendar-offline-operation-checkpoint-2026-11-02).

Prepare requires source repo/full commit/MW, independent binding and committed
ledger flags, and either `--logical-id` or `--old-source-path` / `--old-source-line`,
never both. Nonretired exact current acknowledged/parser-matching open valid-due
source and **no pending anywhere** are required. Enrolled creates reserve `h1` plus
namespace/logical-UUID SHA-256 hex; Active updates retain old event/base etag and
require changed managed payload hash, otherwise no-op refusal. Fresh bounded
OS-entropy UUIDv4 identifies the operation. Only selected pending Prepared is added;
source ACK/watermark/lifecycle/provider ACK are unchanged, with no comments added.
Event reservation is not provider absence, ownership, or API-idempotence proof.

Mark-uncertain generation requires only binding/ledger flags, the same exclusive
selector and persisted `--operation-id`, no source/MW inputs. Only Prepared ->
Uncertain changes; the immutable intent, domain-versioned binary digest (excluding
phase), frozen sourceCommit/parser metadata, and source ACK/watermark remain.
Today's edits/deleted notes/unavailable parser do not block recovery. New generation
from Uncertain refuses, but exact apply retry is idempotent. Generic verify/apply
CLI still requires SourceArgs: `--commit` must match frozen candidate metadata,
while repo/MW paths are not read for this phase-only route. This is not current
source eligibility. No cancel/clear/reset/reprepare/delete/retire route exists;
both phases block new prepare. Persist prepared intent before any future dispatch;
Prepared is not safe-to-retry or dispatch authorization.

Simulation requires frozen binding/ledger/selector/op ID plus absolute
`--observation`, approved 64-lowercase-hex `--observation-sha256`, and required
`--assert-operator-observation`. Strict 1 MiB regular no-follow single capture
rejects duplicate/unknown/trailing JSON. Observation V1 has only operator_asserted
provenance, binding/logical/operation/managed IDs, intent digest, event ID, valid
asserted result etag, and `{summary,startDate,endDate,transparency}` payload.
Frozen bindings/target/digest/payload must match and payload hash is recomputed;
current notes are not read. Fake authenticated provenance/input `remoteVerified`
refuses, with no trust override. Output V2 `calendar-provider-simulation` embeds
typed observation/hash and `hypotheticalLedger`, with `simulationOnly: true`,
`persistenceAllowed: false`, and false remote/publication/head flags. Hypothetical
success sets Active/provider ACK/pending None, retaining frozen source ACK/watermark
even when notes are gone. **Ordinary verify AND apply reject actual simulation
output even with a correct approved SHA**, leaving durable pending/provider ACK
unchanged. At this offline checkpoint no authenticated-success constructor or
transport existed; the private adapter below does not make simulation persistable.

Ledger V1 and strict enroll/reack Candidate V1 wire formats remain unchanged.
Strict V2 `calendar-operation-candidate` prepare/mark_uncertain has full replay and
internal operation proof; V2 state ownership binds op ID/intent/committed pending
phase/event/payload. Exact V1 marker/encoding is unchanged. Current writer reads
both, old state writers refuse V2-owned tips, not necessarily read-only V1 ledger
consumers. New-operation local apply receipt V2 adds operation/intent fields; V1
receipt fields remain unchanged. Same bare fixed-ref CAS/exact retry/cancellation
policy; ledger CAS is not exclusive API dispatch ownership or provider concurrency.

Historical offline main implementation-session macOS results: **415 normal workspace passed,
13 ignored**, serial `cargo test --workspace --locked --offline -- --test-threads=1`;
Calendar **130 normal/3 ignored**; CLI `calendar_apply` **4 normal/2 ignored**.
**All six explicit real-MW gates ran and passed:** two apply fixtures including new
offline operations, candidate, plan, preview, source-reader. The main new CLI flow
ran enroll/apply V1 -> prepare/verify/apply V2 -> global pending block -> remove
current source/parser -> mark-uncertain/verify/apply -> exact retry. Preparation
inputs were restored for true stale-ref CAS rejection with no object changes,
then removed again. Required assertion and actual simulation output's correct-SHA
rejection by both verify/apply were proven with unchanged objects/refs, pending
retained and provider ACK None. Wrong observation binding/IDs/digest/event/etag/
payload/forged provenance were rejected. Strict Calendar all-targets Clippy,
fmt/build passed; review found no local offline P1/P2 and CLI e2e follow-ups are now
done. These tests were executed in the implementation session, not by the user or
this docs pass; older 389/363/339/303 checkpoints remain historical.

Fixtures wrote only temporary repositories, including temporary state Git writes.
No Google API/Keychain/live user-state/hooks/CI/account-quota calls, actual user-repo
Git commit/ref/push, secret/billing/provider changes, installed binary or worker
changes. At this offline checkpoint provider-result persistence was incomplete;
the private local ACK stage below now exists. Production activation still needs
appropriate write OAuth/native or hosted secret storage, authenticated bound event
observations wired through the private durable ACK path, and remote-authoritative
dispatch ownership beyond Git CAS.
Remote CAS push/hosted workflow/billing hard stop and later Linux remain gates.
No paid DB is needed for this protocol. Normal-user UI/policy, automatic enrollment,
and actual activation remain pending; conservative source-basis restrictions are
unchanged. Linux remains explicitly user-deferred with no probes/VMs/targets.

Historical adapter-only checkpoint (private ACK milestone follows):
**Dormant private HTTP adapter code/loopback-test milestone DONE, not an actual
publisher or activation.** Hive now compiles real request builders in private
`calendar_transport`, with no new public export, CLI command/config/env credential
lookup, or production consumer. Its narrow module `allow(dead_code)` documents
intentional nonactivation. POST/PATCH consumes a private move-only permit bound to
binding/IDs/intent digest/kind/origin and Prepared phase; no production constructor
exists. Prepared and local Git CAS do not grant authoritative dispatch ownership.
Opaque zeroizing/redacted `SecretToken` makes no verified claims; no credential
provider, Keychain, refresh, or grant behavior changed.

Before any Calendar request, same-bearer Userinfo `sub` must match the binding's
existing Google identity namespace, not email/token claims/public token fields.
Principal binding is not scopes, calendar ownership or dispatch permission; future
activation requires separate Calendar-write + OpenID authorization, not existing
Tasks-readonly authority. Fixed HTTPS endpoints, HTTPS-only/no proxy/redirect/retry,
shared 30-second operation deadline, 10-second connect timeout and 64 KiB Userinfo /
1 MiB event caps are implemented. Reconcile GET observes only the bound desired
event, not mutation causality. Create sends reserved ID/managed payload/private
ownership tags; update GET validates prior ownership/hash/exact quoted strong etag
before managed-only If-Match PATCH, preserving all unrelated private properties.
No unmanaged-event writes, DELETE, timed-event or recurrence support. Ownership tags
are a hash/UUID convention, not a cryptographic signature; see the source-linked
[adapter contract](../../hive-mind/docs/product-architecture-roadmap.md#calendar-private-http-adapter).

409 is collision for later explicit reconcile, 412 conflict without retry. Sent
mutation 5xx (500/502/503/504 included), I/O/timeout or malformed/mismatched 2xx is
Uncertain; Userinfo/GET 5xx is Unavailable with no write. Static mappings such as
429 do not authorize retry; Prepared still requires reconciliation. Sealed
validator-only success has private fields/no serde/public constructor, binding
principal/binding, IDs/intent/event/etag/hash, mutation-vs-observation cause and
immutable origin. Test-only literal `127.0.0.1` HTTP origin eligibility is false:
it cannot become production authentication proof or a DurableAuthClaim. No ACK
consumer/reducer/persistence, V3 receipt, exclusive worker or remote Git coordination
was added. Simulation remains nonpersistable and Ledger V1, Candidate V1/V2 and
local receipts unchanged. Trusted results cannot come from JSON/remoteVerified flags.

Historical adapter-only main-session macOS validation: **430 normal workspace passed /
13 ignored**, serial `cargo test --workspace --locked --offline -- --test-threads=1`;
Calendar **145 normal / 3 ignored**. All **15 actual reqwest loopback tests** passed
latest default-parallel and serial runs plus full suite. Coverage includes principal
first/mismatch zero Calendar I/O, exact wire data/private-property preservation,
bad IDs/hash/owner/classes/etags, status/collision/conflict/sent-5xx, no redirect
secondary contact, bounded chunked/oversized/truncated/trickled responses, token
privacy/HTTPS refusal and bad permits with zero I/O. Shared-budget assertions are
phase-aware: recorded mutation -> Uncertain; exhausted preflight -> Unavailable
with GET-only evidence. Earlier sent-5xx P2 closed, builder-test delimiter corrected,
timing stabilized; final review no dormant-adapter P1/P2. Strict Calendar all-targets
Clippy `-D warnings`, fmt/build passed; four legacy main-CLI warnings remain.
No new gated real-MW fixtures: **six real-MW gate passes are prior 415-checkpoint
evidence and were not rerun for this adapter**. Only loopback HTTP was tested, not
live Google authentication/provider behavior. Dependencies add declared references
only, no lockfile version upgrades/downgrades. See
[latest evidence](../../hive-mind/docs/release.md#calendar-private-http-adapter-checkpoint-2026-11-02).

No actual notes/refs/state/provider/Keychain/credentials/workflow/billing/secrets,
installation or capture-worker changes. This documentation pass runs no tests,
network/services or state operations. At that checkpoint next work was trusted
in-process result -> durable ACK validation/receipts/recovery, exclusive authoritative dispatch
ownership, credential/grant wiring, hosted CI/remote CAS, cost cap and explicit
activation approvals. Remote API and Git persistence are not one transaction.
Linux is explicitly deferred until later before deployment, with no probes/VMs/targets.

**PRIVATE in-process ACK -> local CAS code/tests DONE, NOT activation.** Hive's
`calendar_transport/ack.rs` pins exact committed base/path/blob, old ledger and full
pending including phase before HTTP. `contextDigest` attaches to intent and sealed
result; Google-only admission matches basis/namespace/IDs/intent/event/hash/strong
ETag and origin. Actual loopback HTTP origin is always refused for durable ACK;
operator JSON never becomes authority. Explicit internal `LedgerChangeProof`
separates Source, Intent and Acknowledgement with private-constructor `SealedAckProof`.
No new ACK/publish command or raw authenticated-JSON path: ordinary public apply
still accepts only V1 source/V2 intent candidates.

Frozen Prepared/Uncertain create/update ACKs do not gate on current source/parser/
task eligibility, and do not support delete. Reduce ONLY selected Active/provider
ACK/result/pending None, preserving CURRENT old source ACK (not intendedSource),
watermark/order/tombstones/unselected evidence. GET `ReconciledObservation` is not
worker-dispatch proof. Borrowed `PinnedAckTransaction` retains original base/result
across errors and rebuilds the same commit from OLD base every retry, even after
latest pending clears; later advancement refuses, without rebase. Existing bare,
fixed-ref/no-worktree/exclusive-admin/CAS/cancellation policies remain unchanged.

V3 owned/apply receipts validate OLD parent ledger/full pending phase/result and
exact new reduction. V1/V2 goldens remain unchanged. All following Source/Intent
writes stay V3, retaining the minimum-reader floor. Audit/provenance floors separate
GoogleAuthenticated from TEST-ONLY SyntheticFixture: test factories force Synthetic
and recompute digests, never Google from loopback; synthetic continuations retain
that floor. Production excludes the variant and rejects fixture branches even
after ACK clears pending. Digests/Git receipts are not cryptographic signatures;
generic publication/remote/ref verification stays false, not current remote sync.

At the ACK checkpoint, dispatch child opaque `RemoteDispatchAuthority` + `FreshAuthoritativeView` had no
production constructors/Serde/Clone; mint consumes matching head/path/blob/context/
owner nonce/intent/origin/Prepared evidence. Google permits require authority basis;
test factories mint loopback only. This is NOT an actual remote lease/fencing:
local CAS/nonces/freshness and single-use values cannot stop a paused old cross-host
worker; there is no expiry takeover. Remote-authoritative safe ownership remains
UNIMPLEMENTED and production minting unavailable. The creator-only seam below adds
fixture-backed acquisition, not production authority. See the
[source-linked ACK contract](../../hive-mind/docs/product-architecture-roadmap.md#calendar-private-ack-persistence).

Historical ACK FINAL GREEN executed by the **main implementation session**, not the user
or this docs pass: DEFAULT-parallel `cargo test --workspace --locked --offline`,
**444 normal PASS / 13 ignored**; Calendar **159 normal / 3 ignored**. Strict
Calendar all-targets Clippy `-D warnings`, fmt/build passed. Six explicit real-MW
compatibility gates RERAN earlier this session and passed (CLI apply x2, candidate,
plan, preview, source). ACK writer tests are SYNTHETIC trusted handoffs/TEMP repos,
NOT a real Google authentication pipeline. Actual loopback admission refusal,
both phases/create/update, unavailable notes/parser, identity/base/phase/source/
mapping/evidence tamper, SHA-1/SHA-256 V3 CAS/exact retry after injected post-CAS
unknown/later-advance refusal, continuation floors and reader compatibility passed.
Focused review found no P1/P2 for private local ACK under bare/exclusive admin,
not production approval. Older 430/415/389 milestones remain historical.

Test-stability history is not all-green: initial full serial failed the pre-existing
capture 100 ms output-cap race. TEST-ONLY cap timeout became 2 seconds, with separate
100 ms timeout case preserved; production capture Runner did not change. Later
default AND serial full runs failed loopback TransientUnavailable before complete
Userinfo (request counts zero). Fake HTTP fixtures now record complete headers/body,
do not consume replies on empty requests, use 10-second stream timeouts, one mutex
per HTTP test (not nested per server), and a cfg(test) shared OnceLock blocking
client/pre-listener warmup. Production HTTP config, error classes, request-count
assertions and retries were not relaxed. Root cause is NOT established: accepted=1/
complete=0 reproduced and ended on cleanup, not lifetime expiry; slow-client-build
theory did not reproduce. After pooling, bounded-response passed ten consecutive
times, 16 HTTP tests including ACK passed twice serial plus parallel module, then
Calendar 159 parallel and MAIN final workspace 444 parallel passed. **No full serial
444 pass is claimed; earlier full serial runs FAILED.** See
[full ACK evidence](../../hive-mind/docs/release.md#calendar-private-ack-checkpoint-2026-11-02).

Remaining gates: remote Git authoritative ownership/fencing, real provider grants/
credential wiring, full worker integration, hosted push trigger plus
remote CAS, cost hard stop and explicit activation approvals. No automatic Calendar
writes now. No actual notes/source/refs/Google/Keychain, private queue/credentials/
services/deployment, installed binary/settings/worker, Git commit/push or secret/
billing changes; implementation writes were TEMP-only. This docs pass runs no tests,
builds or network. Linux remains explicitly deferred until later before deployment,
with no probes/VMs/targets.

**PRIVATE CREATOR-ONLY COORDINATOR SEAM code/tests DONE, not hosted execution or
production readiness.** Hive's private coordinator has a sealed AuthoritativeState
trait with no production backend implementation/constructor, credential wiring,
public export, new CLI or workflow. The dispatch code is a receipt-consuming bridge,
not a usable production factory. Only opaque FRESH successful exact H0 -> H1 CAS in
the SAME invocation can mint ONE intent-bound move-only permit, consuming its
nonce-bound receipt against exact latest H1 head/path/blob/binding/authorityId/origin.
Any existing Prepared/Uncertain pending anywhere in the binding is RecoverOnly
before staging/publication. Own old head, same-job rerun, copied nonce, local
ApplyReceipt/`already_applied`, logs and matching H1 readback NEVER acquire.

Fixture staging checks actual sole-parent H1/H0 objects and exact single prepare,
old no-pending, valid frozen sourceAck/payload and unchanged source/provider ACKs,
watermark/tombstones/order/unrelated evidence. Source eligibility is delegated to
the preparer; the coordinator does not validate fresh notes. Root checks after
CAS/latest view AND after CPU mint discard cancelled/expired Ready as
PublicationUnknown; consumed receipts cannot retry and next invocation is recovery
only. Caller MUST check root before HTTP: not atomic revocation. Ready is an **H1
policy pin with default/empty repository path**, not a materialized local pin. A
future worker MUST materialize/verify H1 and load actual local PinnedContext with a
real repository BEFORE HTTP/durable ACK. Full HTTP + local ACK + remote publication
execution is NOT wired by this slice.

ACK publication is separate, with no production AckPublication builder. Completed
requires confirmed exact authoritative ACK CAS, not local ACK commit, unknown or
conflict. Post-publication root cancellation is unknown; a lost ACK reply may mean
H2 committed/pending cleared, not necessarily unchanged pending. No repeated
provider write is authorized; actual remote ACK readback/recovery is unimplemented.
Synthetic V3 ACK local evidence never becomes Google; actual Loopback results still
fail the private durable ACK gate.

Latest supplied MAIN macOS validation, not the user or this docs pass:
`cargo test --workspace --locked --offline -- --test-threads=4`, **464 normal PASS /
13 ignored**; Calendar **179 normal / 3 ignored**; coordinator **20 tests passed
serially**, including post-mint root cancel/expired Ready discard and next-invocation
no new publication. Strict Calendar all-targets Clippy `-D warnings`, fmt/build passed.
The earlier transport subset passed 43 tests with 19 coordinator tests; current
44-test total was NOT rerun as a whole. Six real-MW gate passes are historical
444-checkpoint evidence, NOT rerun for this slice; no new MW gate. Final main CLI
`clippy --no-deps` remains with MAIN, with four existing legacy warnings expected.
Historical 444 default-parallel and 430/415/389 serial checkpoints above remain
separate. Focused review readiness is private protocol, not publisher readiness.
See the [coordinator contract](../../hive-mind/docs/product-architecture-roadmap.md#calendar-private-creator-only-coordinator)
and [exact validation evidence](../../hive-mind/docs/release.md#calendar-private-coordinator-checkpoint-2026-11-02).

Test-only in-memory and owned TEMP bare authoritative adapters use separate
worker-local bare state, bounded local Git packs and atomic `git update-ref NEW
EXPECTED_OLD`, never ordinary push or network Git. Origins are Loopback only, never
Google. Two H0 creators have at most one CAS winner/permit; lost reply/own-H1 readback
is recovery-only, missing/stale/advanced/non-ABA rollback fails without rewind and
basis/nonce/origin mismatches fail closed. A paused creator may make its late first
attempt, but second invocation never writes. GET 404/410 recovery has two GETs,
zero mutations. No expiry/steal/reset/reprepare-pending/blind retry. Local CAS/nonce/
fresh view is NOT remote lease/fencing/exactly-once; Git ABA/admin rewind is outside
scope, requiring cooperative writers/exclusive admin and later remote controls.
Future actual remote CAS must prove expected OLD at the server atomically, not
unconditional REST ref update, ordinary FF push or readback.

Remaining gates are separately reviewed actual fixed-identity remote exact-CAS
backend, full worker/materialized H1 pin/credential grant, authoritative ACK recovery,
queue/dispatch policy operations, source-policy UI, hosted workflow/pinned Linux
later, account paid-overage hard stop and activation approvals. No actual notes,
refs, GitHub/Google/provider, credentials/user state, installed binaries, capture
worker, services, CI, pushes, secrets or billing changed; no paid service provisioned.
Only loopback HTTP and TEMP bare Git fixtures were used in implementation. Linux
is explicitly DEFERRED, no probes/VMs/targets. This docs-only pass runs no tests,
builds or network and makes no state-ref changes or uploads/upload offers.

Private plan/ledger content and IDs must not be logged or automatically uploaded as
CI artifacts, including Atlassian; the user explicitly declined Atlassian uploads.
The user selected due date only for v1: no start-date requirement; start time and duration
are important future metadata, deferred along with timezone/DST policy. Preview a
date-only deadline as an all-day Available reminder, not a busy block. Local durable
enrollment persistence code is delivered, but integration activation remains pending; do not automatically add task
comments or silently identify unmarked entries to fill those gaps. This first
slice is one-way publication, not bidirectional Calendar synchronization.
See the sibling [hosted publication contract](../../hive-mind/docs/product-architecture-roadmap.md#accepted-next-slice-hosted-calendar-publication-after-push).

- [ ] Start with one recurring review event linking to the usable review surface.
      This is an adoption aid and can be created manually before API integration.
- [x] Deliver the narrow committed-source due-only local preview: validate literal
      checkbox/metadata due values against the parsed date; skip completed/undated
      tasks and diagnose invalid/duplicate dates without events. Valid dates preview
      all-day Available/transparent reminders with exclusive next-calendar-day end,
      not start/estimate scheduling. Unmarked identity remains visibly unresolved;
       no HTML comments are added. Hosted publication remains unchecked below.
- [x] Deliver strict committed-ledger-relative local planning with explicit bindings,
      exact markerless source acknowledgements, ancestry checks, pending retry blocks,
      and retirement evidence. Bootstrap is observation-only, not enrollment.
- [x] Deliver local enroll/reack candidate generation with explicit identity mapping
      and validated whole-ledger proposals only, no persistence or provider dispatch.
- [x] Deliver reviewed local persistence with semantic replay, approved UUID retention,
      dedicated bare state-ref whole-HEAD/path CAS, and exact-retry/cancellation recovery.
      Local code/test milestone under exclusive state-repository administration only.
- [ ] Deliver normal-user approval/UI and actual integration activation; no manual
      ledger-ID maintenance or automatic user-repository apply.
- [x] Deliver offline frozen prepare/mark-uncertain candidates, reviewed V2 local
      persistence, and operator-asserted nonpersistable ACK simulation, not publication.
- [x] Deliver dormant private HTTP adapter code and local loopback tests only;
      no production permit constructor, publisher or activation.
- [x] Deliver private pinned in-process ACK validation/reduction, V3 receipts and
      local CAS/exact recovery, synthetic floors and gated dispatch seam only.
      Code/tests DONE, not activation or real Google authentication-pipeline proof.
- [x] Deliver private creator-only coordinator protocol seam and TEMP backend tests,
      not a production backend, full worker integration or hosted publisher.
- [ ] Deliver full worker integration with materialized/verified H1 local pin and
      real provider grants/credentials, plus authoritative ACK readback/recovery.
- [ ] Deliver queue/dispatch policy operations and source-policy/approval UI.
- [ ] Implement separately reviewed actual fixed-identity remote exact-OLD CAS;
      unconditional REST ref update, ordinary FF push and readback are insufficient.
- [ ] Implement remote Git authoritative ownership/fencing beyond single-use local
      values/CAS; local freshness does not fence a paused old cross-host worker.
      Remote API calls and Git persistence are not one transaction. Implement hosted
      push trigger, CI serialization and remote CAS, not just local ref CAS.
- [ ] Implement Calendar write auth/executor and hosted workflow;
      verify account cost hard stop and obtain deployment/write/secret approvals.
      Validate Linux later before deployment, as explicitly deferred by the user.
- [ ] Automatically publish open undated tasks to one dedicated Google Tasks list
      and dated tasks to one selected Google Calendar. Do not require a separate
      scheduling/export action once the task's date has been supplied.
- [ ] Define which source field supplies the routing date (`due`, `start`, or a new
      scheduled date); distinguish deadlines from planned work. Add explicit timing
      metadata for true time blocks. Do not silently reinterpret existing dates
      during migration. Agree the date-only policy before enabling automatic routing.
      **First-slice decision:** use `due` only, with no start-date requirement, for
      a preview of all-day Available deadline reminders. Start time/duration and
      true timed busy blocks are deferred; broader routing remains unchecked.
- [ ] Use the Calendar API for actual busy blocks. The Google Tasks API records
      dates but cannot read/write scheduled time-of-day; it cannot implement timed
      blocking by itself. Avoid trying to model all-day busy as the default workaround.
- [ ] Add Google OAuth for both APIs, minimum scopes, secure refresh-token storage,
      timezone/DST handling, and durable mappings from stable MindWeaver IDs to the
      Google task/list or event/calendar IDs. Reconcile ambiguous create responses
      before retrying; Google Tasks insertion must not be assumed idempotent.
      Adopt an incoming phone-created Google Task's existing ID rather than
      exporting a duplicate back into the same list.
      **Partial delivery:** Tasks-readonly + openid login/refresh/native macOS storage
      and inbound verified mappings are implemented in Hive. This broader milestone
      remains unchecked: Calendar/write authorization, timezone/DST handling, and
      export/reconciliation are not delivered by that narrow grant.
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

The activated narrow remote path is Google Tasks → scheduled Hive private queue →
the editor-owned shared local capture operation → durable canonical receipt.
Actual new-phone scheduled delivery and repeat nonduplication are live-verified;
computer-off catch-up and pending-job dirty-buffer delivery proof remain pending.
Completion →
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

1. Retain the active Hive inbound/auth setup, existing queue/baseline, and both
   one-attempt receipts. New scheduled phone-to-editor-owned writer delivery and
   repeat nonduplication are verified; next prove live computer-off catch-up and
   pending-job dirty-buffer delivery deferral. See the sibling
   [capture runbook](../../hive-mind/docs/google-tasks-capture.md).
   Broader PWA/offline-client coordination remains separate.
   Local CLI/editor/real-notes cutover is complete; do not request it again.
2. Keep generic index cleanup and optional domain-only activation separate and pending.
   Local checked Done/Reopen selection is delivered.
3. Agree a daily review cue and explicit log-selected-completed action; establish
   the actual life-log layout and a separate reflection view. Immediate archival
   is not required and no automatic archive currently exists.
4. Android is confirmed. Which browser will be used, and is the existing Hive PWA
   installed or any Hive service currently running? Prefer a Google Tasks widget for
   capture, the PWA shortcut, or both? Deployment state has not been verified.
5. Due-only v1 uses all-day Available reminders, not timed busy blocks. Local schema,
   planning, candidate generation, reviewed bare-repository persistence/recovery,
    offline frozen intent/uncertainty/nonpersistable simulation and dormant private
    HTTP adapter/loopback-test, private pinned ACK/local-CAS/V3 receipt and private
    creator-only coordinator seam
   code/test milestones are delivered, not real-repository activation. Continue
    full worker integration/materialized H1 pin, authoritative ACK recovery,
    queue/dispatch policy operations, real provider grant/credential wiring,
    normal-user approval/UI and integration activation, and remote Git authoritative
    dispatch ownership/fencing beyond single-use values and local CAS,
   Calendar OAuth grant/executor, hosted CI serialization and remote lease/CAS push,
   billing hard stop, and explicit deployment/write/secret approval gates. No actual
   user-repository apply is approved by these docs. Linux artifact/runtime validation
   is user-deferred until later before deployment, not blocking local work. Broader routing is pending:
   date → Calendar; no date → Google Tasks.

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

- [Google auth setup and validation boundary](../../hive-mind/docs/google-auth.md)
- [Inbound capture runbook and queue V2 safety](../../hive-mind/docs/google-tasks-capture.md)
- `plan.md` and `docs/product-architecture-roadmap.md`
- `hive-pwa/src/App.svelte`, `hiveStorage.ts`, and `hiveSyncEngine.ts`
- `hive-pwa/public/sw.js`
- `crates/hive-desktop/src/lib.rs` and `crates/hive-server/src/lib.rs`
