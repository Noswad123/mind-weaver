# CLI Reference (Quick)

## Core commands

- `mw -help`
- `mw version --short`
- `mw init --notes-dir <path>`
- `mw doctor`
- `mw config path`
- `mw config show`
- `mw config json`
- `mw notes sync`
- `mw notes seal`
- `mw seal` (shortcut for `mw notes sync`; Rust `mw sync` is unsupported)
- `mw notes format --all`
- `mw notes meld --all`
- `mw format --all` / `mw meld --all`
- `mw notes ingest --prune`
- `mw notes banish --prune`
- `mw ingest --prune` / `mw banish --prune`
- `mw notes get --search <query>`
- `mw get --search <query>` / `mw summon --search <query>`
- `mw notes validate --all`
- `mw validate --all`
- `mw notes validate-registry`
- `mw notes validate-db`
- `mw validate-registry`
- `mw register`
- `mw graph` / `mw loom`
- `mw notes issues`
- `mw notes watch --fg|--status|--stop|--restart`
- `mw notes validate --domain <domain>`
- `mw query notes --uid <uid>`
- `mw query notes --uid <uid> --format text --view commands`
- `mw query registry`
- `mw query ids --uid <literal-id>`
- `mw query ids --prefix <literal-prefix> [--limit 50]`
- `mw query domains`
- `mw query todos`
- `mw query projection recipe`
- `mw query ingredients`
- `mw query ingredients --mentions`
- `mw todos add "text" [--capture-id <32 lowercase hex>] [--json]`
- `mw todos migrate plan [--json]`
- `mw todos migrate commit --plan -`
- `mw todos sync [--require-task-safety-v1]`
- `mw todos snapshot --json`
- `mw todos done --id <todo-id> --expected-revision <sourceRevision> [--json]`
- `mw todos reopen --id <todo-id> --expected-revision <sourceRevision> [--json]`
- `mw todos toggle --id <todo-id> --expected-revision <sourceRevision>`
- `mw todos inspect --id <todo-id>`
- `mw todos update --id <todo-id> --expected-revision <sourceRevision> --priority p1 --due YYYY-MM-DD`
- `mw todos identify --id <todo-id> --expected-revision <sourceRevision>`
- `mw todos refresh [--discard-dashboard-edits]`
- `mw todos archive plan|commit|inspect|recover` (explicit checked archive protocol)
- `mw` / `mw tui notes` / `mw tui todos`

`mw` embeds its default SQLite schemas; `SCHEMA_PATH` / `NOTES_SCHEMA_PATH` are
only needed when intentionally testing an alternate schema file.

Rust note validation covers filesystem/registry ID checks, DB-backed registry
conflicts, and embedded domain-schema checks via `mw notes validate --domain`.

`mw notes watch` runs in the background by default and supports foreground mode
and process management:

- `mw notes watch`
- `mw notes watch --fg`
- `mw notes watch --status`
- `mw notes watch --stop`
- `mw notes watch --restart`
- `mw notes watch --format` to include formatting in the refresh pipeline

## Machine-readable config

`mw config json` is a read-only Rust engine endpoint. On success, stdout contains
only a JSON object with these exact fields:

```json
{"schemaVersion":"1","notesDir":"/absolute/notes","inboxPath":"/absolute/inbox.md","dashboardPath":"/absolute/dashboard.md"}
```

It uses the existing config loader, including `MW_CONFIG`, environment overrides
(`MW_INBOX_PATH` takes precedence over `INBOX_PATH`), and loader defaults when the
config is absent. An existing config suppresses dotenv loading. Home/environment
expansion follows the engine; remaining relative paths are joined to the process
working directory, not the config directory or notes root. Paths need not exist
and are not canonicalized. Overriding only `NOTES_DIR` does not relocate the
engine's default inbox or dashboard under `~/Notes`.

No directories or databases are created, no token commands are run, and no other
config fields or secrets are serialized. Invalid config exits nonzero without a
JSON fallback. Human-facing `mw config show` and `mw config path` are unchanged.

## Local task workflow

### Overview

Local cutover DONE (2026-10-30): the release CLI is installed, the live Neovim
integration is upgraded, and the reviewed migration of 12 real sources plus the
dashboard succeeded (`modifiedPaths`: 13, `projectionUpdated`: true). A fresh default
plan proposes zero changes and `blockers: []`. Six preparatory repairs preceded
this cutover. Hive cutover, Google routing, Android
delivery, and native widgets are not implemented by this slice.

Real review sync passes (7 dashboard tasks, 1,494 Markdown files, 12 active sources).
The snapshot has 11 tasks: 7 Do Now and 4 Inbox across 6 nonempty notes; zero completed
and zero stable IDs. Migration does not assign existing task IDs. No test tasks,
completion, or archival were performed in real notes. Global read-only
`notes validate --all` still fails on the pre-existing duplicate ID
`work-reinvention-pm-enablement` in
`moderna/moderna-it/work-reinvention/work-reinvention-pm-enablement/{hub.md,work-reinvention-pm-enablement.md}`.
No broad ingestion/database refresh or `.mw` registry rewrite ran; DB-cached note
queries may retain old metadata until deliberate refresh.

Active task sources require **both** `domains: [task]` and `task_active: true`.
Domain membership alone does not enable tasks; simplifying activation is deferred.
Use `## Todo` with `### Do Now`, `### Inbox`, and `### Blocked`. Only **Do Now**
projects onto the dashboard; Inbox and Blocked remain source-backed tasks available
to snapshot/query. Active legacy `task-index`/`taskIndex` domains and Next/Waiting
headings are rejected with migration guidance, not accepted as runtime aliases.

### API / Interfaces

`mw todos add "text" [--capture-id <32 lowercase hex>] [--json]` captures a single
nonblank title into the configured `inbox_path` (`INBOX_PATH`, overridden by
`MW_INBOX_PATH`). The destination must be a Markdown file under the canonical notes
root, distinct from the dashboard. The notes root and source/dashboard parent
directories must already exist. Relative config paths retain the config loader's
cwd-relative semantics; capture does not implicitly relocate an external Inbox.

A missing or zero-byte destination gets a canonical active task scaffold, note ID,
and all three sections. An existing nonempty destination must already be an active
`task` source with one `## Todo` and unique canonical Do Now/Inbox/Blocked sections;
capture does not silently repair or migrate it. The new task is inserted immediately
after `### Inbox` with a stable identity marker. JSON output is
`{schemaVersion: "1", task: TaskIndexTodo, alreadyCaptured, projectionUpdated}`;
without `--json`, output is a human-readable capture summary.

`mw todos migrate plan [--bootstrap-legacy-dashboard] [--json]` always emits a pure-read JSON report, without
creating locks, backups, directories, or a projection. `TaskMigrationPlan` contains
`schemaVersion: "1"`, `kind: "task-migration-plan"`, `notesDir`, `dashboardPath`,
`dashboardRevision`, `bootstrapLegacyDashboard` (defaults to false when omitted),
`observations` (source paths to revisions), `changes` (each
`sourcePath`, `sourceRevision`, full `replacement`), and `blockers`.

`mw todos migrate commit --plan -` accepts exactly one strict JSON plan on stdin
(32 MiB maximum; required fields, no unknown fields or trailing JSON). It rederives
the plan and requires exact equality with the reviewed report and no blockers.
Success emits `schemaVersion: "1"`, `kind: "task-migration-result"`, `modifiedPaths`,
`backupPath`, and `projectionUpdated`. An unchanged empty plan writes nothing and
returns a null backup path.

`--bootstrap-legacy-dashboard` is a reviewed migration option, not a discard flag.
It permits an unversioned generated dashboard only when every row uniquely matches
a prospective source task by source ID, exact text, Do Now ownership, area, and
completion state. Existing rows may be a stale subset; unknown or duplicate rows,
checkbox differences, prose/body edits, malformed metadata, and reserved protocol
markers block the upgrade. Corrupt/versioned dashboards cannot fall back to legacy
parsing. The plan binds the opt-in and original dashboard bytes; commit backs up
that dashboard and publishes its supported v1 replacement last. Dashboard-only
upgrades also use the guarded backup/publication path.

`mw todos done --id ID --expected-revision HASH [--json]` and analogous
`mw todos reopen` set the desired completion state explicitly. Both require the
revision actually observed by the caller and dashboard preflight. Setting an
already-matching state is a no-write success only after those checks; a stale
revision or pending dashboard edit still fails. JSON is the updated `TaskIndexTodo`;
human output explicitly distinguishes done from archived. Neither command archives
tasks or restores work already removed into a ledger.
An already-matching no-op confirms source state, not projection freshness; after a
source-committed/projection-failed error, inspect and explicitly refresh the clean
dashboard rather than treating another successful `done` as projection repair.

#### Installed Neovim integration

Capture uses one title prompt, no metadata questions, a cryptographically generated
stable capture ID, and an engine capability probe **before any save**. Arguments use
`--` before the title so option-shaped titles stay literal. The writer pins resolved
paths with `NOTES_DIR`, `MW_INBOX_PATH`, and `DASHBOARD_PATH`; config-file drift cannot
silently redirect a confirmed operation. Dirty-buffer consent covers the whole notes
root, followed by safe reloads. The retired `ops.edit_file` fails closed, including
callbacks retained by older modules.

`MWTodoDone` / `MWTodoReopen` select tasks with their observed revisions. Successful
completion/reopening only notifies; raw output is shown for errors/conflicts. Neither
auto-archives. Mappings are `<leader>nc` capture, `<leader>nx` done, `<leader>nr` review,
and `<leader>na` checked archive. The current session was hotloaded with targeted
reloads preserving the shared ops table; no restart is needed. Future starts read the
active source symlink. Unrelated modules and dirty dotfiles were not reset/reloaded.

### Data Flow

Capture validates title/identity, takes the notes-root lock, preflights the
dashboard, scans saved sources and managed archive history under the notes root,
then prepares/rechecks a source insertion and dashboard projection. Source writes
precede projection publication. No database sync or notes-ingestion pipeline runs.

Migration transforms `task-index`/`taskIndex` to `task` and Next/Waiting to
Do Now/Blocked only within actual Todo ownership. It preserves comments, fenced
examples, inactive status, task markers, completion, metadata, child prose, and
newline style. Complex YAML or ambiguous heading structures produce blockers,
not guessed rewrites. It does not assign stable IDs implicitly. Positional IDs and
source revisions must be observed again after structural edits.

Commit saves the reviewed plan, original sources, and any dashboard under
`<notesRoot>/.mw/task-migration-*`, then performs guarded per-source replacements
and publishes the projection **last**. This is not multi-file atomic or an
external-editor compare-and-swap. Partial errors report modified paths, backup
directory, and backup durability; there is no automatic rollback or blind retry.

### Edge Cases

- Reusing a capture ID with the same **literal title and destination** does not
  duplicate a task; a changed title/destination is rejected. Same title with a
  different ID is a separate task. Omitting the ID generates a fresh one. A retry
  does not reopen an already completed task and may repair a clean stale projection.
- Capture IDs already archived in managed history **under the notes root** are
  blocked. External ledgers are not inspected. Removing both the source identity
  and its managed history loses the retry receipt; IDs are not permanent tombstones.
- Control characters, checkbox-shaped titles, and reserved protocol markers are
  rejected. Symlink files, destination aliases, and Unix hard-linked capture files
  are refused; excluded `.git` destinations, unfinished fences, and ambiguous Inbox
  content also block capture.
- Capture and ordinary migration require a missing dashboard or a clean supported
  v1 dashboard. Reviewed migration may additionally upgrade a fully source-matched
  legacy dashboard with the explicit option above; capture never gets that bypass.
  Pending checkboxes and invalid dashboards still block them, even on capture
  retries. Preserve edits: resolve pending writeback using a compatible old
  engine **before cutover**, then stop writers and review a fresh migration plan.
  Do not delete the dashboard or silently discard edits to get past a blocker.
- Incomplete managed archive operations block migration. Frozen archive V1 history
  is not rewritten; receipt-bound historical inspection/recovery remains supported
  without enabling legacy domains/headings in ordinary runtime reads.
- Stale observations and source-committed/projection-failed errors require inspection
  and a deliberate recovery decision, not a new ID or automatic retry.
- Rollback must coordinate notes, dashboard, editor, and binary: **never restore the
  old binary alone**, because it ignores the new domain. Compare current bytes with
  stored post-state and backups, preserve subsequent user edits, and explicitly
  approve a coherent restoration. No automatic rollback or blind tar extraction.
  See the [operational record](todo-workflow-roadmap.md#completed-local-cutover-2026-10-30).

### Example usage

```sh
# Save and review the full report; commit only after all blockers are resolved.
mw todos migrate plan --json > reviewed-task-migration.json
mw todos migrate commit --plan - < reviewed-task-migration.json
# For an unversioned dashboard, explicitly request source-checked bootstrap:
mw todos migrate plan --bootstrap-legacy-dashboard --json > reviewed-task-migration.json
# Review the new report; never blindly pipe a fresh plan straight into commit.

# Retain this ID for intentional retries of this exact capture.
mw todos add "Call dentist" --capture-id 0123456789abcdef0123456789abcdef --json
mw todos snapshot --json
mw todos inspect --id 'task:0123456789abcdef0123456789abcdef'
# Use the sourceRevision just observed; completion does not archive the task.
mw todos done --id 'task:0123456789abcdef0123456789abcdef' --expected-revision '<observed-sourceRevision>' --json
```

Completion and logging are separate actions. Recommend an explicit daily review:
review Inbox/Do Now/Blocked, then select completed tasks for checked archive
plan/commit into a chosen managed life-log ledger. There is no automatic archive;
bare `mw todos archive` is unsupported.

### Related files

- [Workflow roadmap and pending cross-device work](todo-workflow-roadmap.md)
- [CLI dispatch](../crates/mw/src/main.rs)
- [Task types and scanning](../crates/mw-notes/src/todos.rs)
- [Capture](../crates/mw-notes/src/todos/safety/capture.rs)
- [Migration](../crates/mw-notes/src/todos/safety/migration.rs)
- [CLI workflow tests](../crates/mw/tests/todos_workflow.rs)

## Task Safety

`mw todos snapshot --json` (also JSON by default without `--json`) is the versioned,
read-only endpoint for source-backed task pickers. Its exact envelope is:

```json
{"schemaVersion":"1","source":"saved-markdown","notesDir":"/absolute/notes","dashboardPath":"/absolute/notes/dashboard.md","items":[]}
```

`items` contains full `TaskIndexTodo` objects, including `id`, `stableId`,
`identityKind`, `sourceRevision`, `sourceId`, `sourcePath`, `noteTitle`, `taskScope`,
`todoSection`, `area`, `text`, `done`, `order`, `line`, and `metadata`. Configuration
is loaded once; both provenance paths are absolute `cwd.join(configuredPath)`
values from that same configuration used for the Markdown scan. Paths are not
canonicalized. This reads saved files, not unsaved editor buffers. It does not
open/init a DB, create directories or locks, generate markers, or validate/write
dashboard baselines. A dashboard under the notes tree can be encountered by the
ordinary Markdown scan. Missing notes or malformed/duplicate task identities
return nonzero with no JSON fallback. Database/schema paths may be absent or
unusable without affecting the snapshot. The old `query todos` contract and the
human-facing bare `mw todos` output are unchanged.

Picker integrations should capture the executable/cwd/environment, invoke this
endpoint without a separate config probe, validate the envelope and task fields,
and carry the selected `id` and `sourceRevision` unchanged through selection.
Before any buffer save or mutation, re-resolved notes/dashboard paths must still
match the snapshot provenance. An older engine lacking this subcommand must fail
closed before opening the picker or saving buffers.

Task reads (`query todos`, `todos inspect`, list/TUI scans) never assign identity
or rewrite Markdown. Source frontmatter IDs remain authoritative, not indexed DB
IDs. Duplicate active source IDs or globally duplicated stable task IDs fail the
whole scan. Scan errors propagate to the TUI instead of producing an empty list.
Task reads and checked mutations use a task-specific top-level YAML reader for
`id`, `domains`, `task_active`, `task_area`, `task_default_priority`,
`task_default_energy`, `title`, and `task_scope`. Keys appearing inside a literal
or folded description scalar cannot override these fields, activate an inactive
note, or change task ownership. Plain/comma-separated domain strings and YAML
string lists (flow or block) are supported. Activation accepts booleans and the
legacy scalar true/false, yes/no, on/off, 1/0 forms. Other operational scalar
fields accept strings, numbers, and booleans; collections/tagged values are
rejected rather than flattened for active task sources. Activation/domain
selection occurs before reading the remaining task fields, so ordinary inactive
note/dashboard metadata is not interpreted as task settings. Task scans validate
encountered frontmatter as YAML mappings and fail closed on malformed YAML or
unsupported types in the operational fields they read.
This reader is deliberately task-specific: the global note metadata parser,
indexing, and other note workflows have not been migrated in this slice.
Task parsing excludes the complete top-level YAML frontmatter region as well as
fenced code examples. Todo headings, checkboxes, and reserved marker text inside
literal/folded metadata scalars are examples, never operational tasks or task
identities. Only column-one frontmatter fences delimit this region; an indented
`  ---` inside a scalar does not close it. Task line coordinates still refer to
the saved source, and revisions still hash all exact bytes, including metadata
and CRLF. Active sources with malformed frontmatter detected by the task mask
fail closed rather than allowing metadata edits through a task ID.
Malformed, duplicate, or orphan reserved task
markers in active sources are errors.

An optional standalone indented child comment carries task identity:

```markdown
- [ ] Ship the change
  <!-- mw-task:v1:0123456789abcdef0123456789abcdef -->
```

`todos identify --id <observed-id> --expected-revision <sourceRevision>` assigns
one 16-random-byte identity explicitly and prints the updated `TaskIndexTodo` JSON.
An already identified task is a no-write success only after revision validation.
There is no bulk identity assignment or implicit generation during reads, sync, or refresh.
Toggle/update preserve the marker, including metadata clears. Reserved markers
cannot be injected through title or metadata options.

`TaskIndexTodo` (inspect/identify and library results) adds `stableId` (32 lowercase
hex characters or null), `identityKind` (`stable` or `positional`), and
`sourceRevision` (`sha256:` plus 64 lowercase hex characters). The same field names
are added to `query todos`; existing query fields and their casing, including
`sourceID`, remain unchanged. Marked public IDs are `task:<stableId>`; legacy IDs
remain `sourceId:line:order`. Older serialized `TaskIndexTodo` objects deserialize
with null stable ID, positional identity kind, and empty revision. Revisions hash
exact source bytes, including CRLF and final-newline presence.

`todos toggle --id <id> --expected-revision <hash>` and
`todos update --id <id> [--id <id> ...] --expected-revision <hash>` require the
revision actually observed by the caller. Existing update options are unchanged:
`--title`, `--area`, `--priority`, `--energy`, `--weight`, `--due`, `--start`,
`--estimate` (alias `--est`), `--metadata`, and repeatable `--clear`. Title is
single-target only. One revision supports only one source file; cross-source bulk
updates fail before writing. Every modified target is returned by the library,
even when metadata insertion shifts positional IDs. LF/CR/NUL options are rejected.
Mixed recognized metadata/prose child lines are rejected rather than losing prose;
unrecognized children are preserved. Never fetch a newer revision and blindly
retry: inspect/query again, show the error and changes, and require a new decision.

The dashboard protocol has one `<!-- mw-dashboard:v1 -->` header, a layout marker
immediately after it (ignoring blank lines), and a per-checkbox
`<!-- mw-dashboard:v1:<hex> -->` immediately before its trailing visible
`[[sourceID]]` link. Hex encodes a JSON object with exactly `taskId`, `sourcePath`,
`sourceId`, `sourceRevision`, `done`, and `text`. Readers validate the version,
shape, uniqueness, and visible baseline text/link. Only checkbox edits write back;
title/link edits, malformed/duplicate rows, and legacy/unknown versions fail closed.
No text/occurrence fallback exists.

The layout marker is `<!-- mw-dashboard-layout:v1:sha256:<64 lowercase hex> -->`.
Its SHA-256 input is the nonblank generated body lines in their existing order,
joined with a single LF and **no final LF**, excluding the layout marker itself.
The header, title, area headings, row metadata, visible text, and source links are
all included. Only visible generated row prefixes `- [x] ` / `- [X] ` normalize
to `- [ ] `; baseline JSON's `done` value is not normalized. Blank lines and LF
versus CRLF do not affect this digest; other whitespace does. Frontmatter is not
part of the digest and remains preserved separately.

Readers recompute and validate the marker before any source writes. Deleting,
reordering, or moving a row across areas, changing baseline metadata, or corrupting
the header/manifest fails closed even when another checkbox has pending changes.
This is accidental-edit integrity, **not authentication**: it does not protect
against someone deliberately rewriting the digest. Nonempty legacy v1 projections
without the layout marker require explicit `refresh --discard-dashboard-edits`;
there is no implicit upgrade or inference. A header-only empty bootstrap remains
valid, and newly generated empty projections include the digest and fixed headings.
Explicit discard bypasses body/layout integrity, never frontmatter validation.
This digest is dashboard-body metadata, not a new field in the snapshot JSON
envelope or in `TaskIndexTodo`; those endpoint shapes are unchanged.
Task areas remain source-authoritative: dashboard row movement is not an area
writeback, and regeneration uses current source grouping.

The dashboard body accepts only the generated header, `# Dashboard`, the fixed
focus-area headings in their generated order, blank lines, and baseline-bearing
checkbox rows under those headings. A header-only empty dashboard is also valid.
Prose, custom headings, incomplete/reordered headings, and other unsupported body
edits are rejected by sync, refresh, toggle, update, and identify before source
writes, including otherwise idempotent identify calls. Explicit
`refresh --discard-dashboard-edits` may discard the body, not metadata.
Dashboard frontmatter is outside this body grammar and is preserved byte-for-byte,
including LF/CRLF and custom YAML literal/folded scalar values. Only column-one
`---` fences delimit frontmatter; indented scalar content such as `  ---` does not.
Malformed/unterminated frontmatter is an error even with explicit body discard.

Nested checkbox tasks in active task blocks are unsupported in this slice and
fail the entire read/scan before mutation or marker assignment. Move child tasks
to separate sibling blocks. Markers beneath nested checkboxes cannot attach to
the parent. The shared checkbox-like barrier covers `-`, `*`, `+`, and numeric
`.` / `)` list markers, with `[ ]`, `[x]`, or `[X]`. Only flat `-` / `*` task
blocks are operational; top-level `+` and ordered checkbox forms in Todo sections
are explicitly rejected rather than silently ignored. Fenced nested examples are
ignored, and ordinary child prose and
non-checkbox metadata children remain supported.

`todos sync` preflights every row before any source write. An unchanged checkbox
does not write its source: current source edits, moves, or deletion win and the
projection regenerates. An edited checkbox requires the unique existing task,
matching path/source ID, and exact baseline revision. At most one source file may
have pending checkbox changes per sync; unchanged rows may span many sources.
Multi-source pending edits are rejected without changing source/dashboard bytes.

Integrations should invoke `mw todos sync --require-task-safety-v1`. This is a
capability fence: older engines that lack task safety reject the unknown flag
before their unsafe sync can run. It is **not a force override** and never bypasses
validation. This engine always applies task-safety-v1 semantics, with or without
the flag. The flag is accepted only by `todos sync`, takes no value, and does not
add a read-capability endpoint. Toggle/update/identify already fence older engines
through their required revision flags.

`todos refresh` is projection-only and refuses pending checkbox edits or an
unsupported/legacy dashboard. `todos refresh --discard-dashboard-edits` explicitly
discards those dashboard edits or bootstraps a legacy dashboard, never writing
sources. Missing dashboards and valid v1 empty headers may bootstrap normally.
Direct toggle/update/identify preflight the dashboard, refuse pending edits, and
regenerate projection only; they do not perform checkbox writeback. The configured
dashboard parent directory must already exist. Identifying an already marked task
does not replace the dashboard.

Mutations use a cooperative `notesRoot/.mw/todos.lock` created exclusively, with
PID diagnostics and Drop cleanup; stale locks are never removed automatically.
The notes root must already exist as a directory; lock acquisition never creates
a missing notes root.
Affected sources and dashboard are snapshotted, prepared before commit, and
exact-byte rechecked. Replacement uses same-directory temporary files, preserving
existing file permissions. Source/dashboard symlink files are rejected (a notes
root or parent directory alias is allowed). This is **not true compare-and-swap**
against non-cooperating external editors: an edit between the last check and rename
can still race. This is also **not a cross-file transaction**. Source commit occurs
before projection replacement. If that later step fails, the error explicitly says
`source committed; dashboard projection NOT updated`, preserves newly observed
external dashboard edits, and warns that blind retry is unsafe.

### Checked Archive

Bare `mw todos archive` is a parser error. The old library function
`archive_completed_to_life_log(notes_dir)` remains disabled. There is no
all-completed selection, automatic ID assignment, pre-sync, force/discard option,
legacy import, or automatic retry/recovery.

```sh
mw todos archive plan --id ID --id OTHER_ID --expected-revision sha256:HASH --archive-path ./ledger.md --json
mw todos archive commit --plan - < confirmed-plan.json
mw todos archive inspect --archive-path ./ledger.md --operation-id HEX --json
mw todos archive recover --inspection - < confirmed-inspection.json
```

Plan and inspect always emit JSON (`--json` is optional) and are pure reads:
configuration is loaded once, notes/dashboard/ledger are read, and no database,
lock, directory, archive artifact, source, or projection is created or updated.
Commit/recover also load configuration once and never access the database. Stdin
must contain exactly one JSON object, with supported version/kind, all required
fields and no unknown fields, including inside task DTOs. Commit/recover accept
only `-`, not a filename, for their input option.

The archive path is literal UTF-8, relative to the process working directory when
not absolute. The engine does not expand `~` or environment variables or lexically
collapse `..` across symlinks. It canonicalizes the existing parent then appends
the final filename; a missing parent, symlink/nonregular final file, or Unix file
with multiple hard links is refused. The destination may be inside or outside the
notes root. Source/dashboard/destination aliases are refused. An archive is either
absent or an exact supported managed ledger; arbitrary month notes cannot be
imported. Prompt-based clients should require a literal absolute or relative path,
not promise shell expansion.

Plan JSON has exactly these fields: `schemaVersion: "1"`,
`kind: "task-archive-plan"`, `operationId` (random 32 lowercase hex, not a task ID),
`notesDir`, `dashboardPath`, `archivePath` (canonical identities), `sourcePath`
(normal root-relative), `sourceId`, `sourceRevision`, `postSourceRevision`,
`dashboardRevision`, `archiveRevision`, `taskCount`, `rawByteCount`, and `items`.
Revisions are `sha256:` plus 64 lowercase hex; absent dashboard/archive revisions
are the literal `absent`. Each item is `{task, byteStart, byteLength,
blockRevision}`, where `task` is the frozen V1 saved task context, including source
defaults, scope, section, area, identity and source revision. Items are sorted in
original byte order. Raw task bytes are not embedded in the plan.

The archival wire schema is owned by `ArchiveTaskContextV1` and
`ArchiveTodoMetadataV1`, not the live task-index DTO serializer. V1 freezes the
existing 15 task fields and 13 metadata fields, their JSON names, declaration
order and types. All fields are required, including `stableId` (explicit null for
positional identity); unknown and duplicate fields are rejected. Live source
tasks are explicitly converted into this known V1 context at planning time.
Source bytes are always preserved independently of the context schema. Future
archival task/metadata schema evolution requires an explicit new wire version,
not silent changes to V1 when live task DTOs gain fields. Finite floating-point
context values use round-trip-safe JSON parsing, preserving their original bits
and canonical receipt representation, including historical effective weights.

`task.metadata.effectiveWeight` is clock-derived snapshot/display metadata, not
authorization. Commit and copied/pre recovery compare reconstructed plans ignoring
only that derived value, so due/start multiplier changes across dates cannot strand
an unchanged source. The original confirmed plan and receipt retain their original
weights; they are never rewritten with today's values. All other fields, including
raw metadata, due/start dates, priority/energy, explicit weight configuration,
identities, spans and revisions, remain strictly checked. Confirmed inspection
comparison is still exact, including its receipt plan and observed file revisions.

Selection is 1..256 explicit unique completed IDs in exactly one observed source,
with a mandatory exact source revision. `rawByteCount` is the sum of actual owned
payload bytes. Intervals contain the checkbox line and indented children, markers,
comments and fenced examples, including blanks between owned children, but not
trailing separator blanks before unrelated blocks. Mixed newlines, tabs, Unicode,
and missing final newlines remain byte-identical. Unclosed or ambiguous owned
fences and unsupported immediate unindented lazy prose are refused. Unfenced
indented headings, including shared-parser `* prose` headings, are also refused
rather than claiming their following tasks as children. Headings inside owned
fences remain raw example payload. Separately
occurring identical tasks are never text-deduplicated. Pruning removes only the
selected intervals; survivors are reparsed with the shared metadata/marker rules
to recompute line/order, positional IDs, metadata and revisions. Stable IDs remain
unchanged. A survivor proof checks that no unselected task line lies in a removed
interval and that every survivor retains its identity/context with exactly the
expected line/order/revision shifts, not merely an expected task count. Archive
never assigns task IDs.

Commit binds the immutable plan semantics described above and all three observed
revisions. It takes
the existing notes-root lock and a canonical destination sidecar lock named
`.FILENAME.mw-archive.lock`, in deterministic path order using nonblocking exclusive
creation. This destination lock serializes even different notes roots sharing one
ledger. Lock files are cooperative and never automatically removed as stale.
The source, ledger and projection outputs are all prepared before the first
replacement. Commit publishes the copy/receipt (`archiveCopied`), reopens and
verifies exact bytes, rechecks all observations, prunes the source preserving its
permissions, records `sourcePruned`, publishes only the guarded survivor
projection, and then records `completed`. Every ledger phase replacement preserves
the raw payload bytes. Incomplete operations block further appends.

The managed Markdown ledger has an inactive task-source frontmatter and a unique
note ID minted only on the first commit. It holds hex-JSON receipts containing the
immutable plan and mutable phase, followed by raw payloads addressed by explicit
byte lengths and SHA-256. Payloads may contain fake protocol comments: parsers do
not search them for delimiters. Exact managed prefix, receipt canonicalization,
operation uniqueness, history revisions, span bounds, UTF-8, lengths and hashes
are validated. Unsupported edits, incomplete framing and corruption fail closed.
Digests provide integrity checks, not authentication. Full-ledger rewrites on each
phase are intentional.

Completed history is portable, but operations are not relocated. Renaming or
moving a fully completed managed ledger does not invalidate its framing, hashes
or history chain. A fresh explicit plan can append at the new canonical
destination using the current configured root/dashboard and exact observed
archive hash; original receipts, note ID, payloads and prior record bytes remain
unchanged. Successful ledger decoding alone never authorizes pruning. Inspect
and recover of an old operation still require that receipt's original archive
path, notes root and dashboard context. They never silently rebase those bindings.
A moved incomplete ledger cannot accept appends or recover at its new location;
restore its original bindings to recover, or await a future explicit relocation
protocol. Older development binaries with codec-global path checks may reject
otherwise valid moved completed history, including after a fresh append here.

The top-level `mw-archive-` plus 32-lowercase-hex note ID is an engine-reserved
namespace and remains the stable document-ownership prefix for future archive
protocols. Engine formatters treat these documents as opaque exact bytes before
any generic metadata, title, heading or hub rewrite. This applies to regular-note
and `hub.md` destinations, the public heading normalizer, `notes format --all`,
seal, and watch's shared `--format` path. Formatting does not validate, repair or
reinterpret a ledger body: corrupt and unsupported-version bodies under the
reserved ID are preserved too. Ordinary notes with nonreserved IDs and fenced
archive-marker examples still format normally. Invalid UTF-8 may cause formatting
to refuse the file, never rewrite it. An archive outside the notes root is not
visited. Third-party formatters and manual edits are outside this protection.
Generic formatting may still change active source revisions; existing receipt
inspection then reports a changed source, while a newly observed explicit plan
can target that source without altering the historical ledger.

Inspection JSON is `{schemaVersion: "1", kind: "task-archive-inspection", plan,
phase, sourceRevision, archiveRevision, dashboardRevision, sourceState,
nextAction, blockedReason, projectionStatus}`. Phase is `archiveCopied`,
`sourcePruned`, or `completed`; source state is `pre`, `post`, `changed`, or
`missing`; projection status is `current`, `stale`, or `blocked`. `nextAction` is
`resumePrune` only for a verified copied/pre operation, `refreshOnly` for post-state
phase/projection repair, or `none`. `none` with null `blockedReason` means complete;
a reason string means blocked. Corrupt ledgers error instead of returning an
action. Configured canonical root/dashboard must match the receipt; inspect does
not silently follow another profile.

Recovery compares the complete confirmed inspection under both locks, binding
source, archive and dashboard revisions. It never adopts newer observations.
Copied/pre recovery reconstructs the original plan using its original operation
ID and compares every authoritative field and exact payload before pruning,
excluding only clock-derived effective weight as described above. In the
post-state, recovery **never resolves or prunes old task IDs**, which could now
identify positional survivors. Pruned/completed operations whose source has been
restored to the pre-state are blocked. Changed/missing sources preserve evidence.
Pending, malformed or legacy dashboards block recovery without discard. A complete
post-state/current-projection operation writes nothing; a clean stale projection
can be explicitly refreshed.

Successful commit/recover JSON is `{schemaVersion: "1", kind:
"task-archive-result", operationId, archivePath, sourcePath, phase: "completed",
sourcePruned, projectionUpdated, alreadyComplete, taskCount, rawByteCount,
sourceRevision, archiveRevision, dashboardRevision}`. `sourcePruned` reports
whether this invocation pruned; `projectionUpdated` reports whether this invocation
published a projection; `alreadyComplete` means no phase/projection work was
needed. Revisions describe the completed source and current ledger/dashboard.

Explicit limits: 256 tasks, 8 MiB source/owned payload, 2 MiB decoded JSON per
receipt, 64 MiB/256 operations per ledger, and 4 MiB CLI stdin. Limits error, never truncate success.
Hash/recheck plus atomic replacement is **not external-editor CAS or a cross-file
transaction**. A non-cooperating editor can race the last check/rename. Partial
errors include operation ID, archive path, last confirmed durable receipt phase,
which remaining steps are not confirmed, and an inspect/recover instruction.
Never blindly rerun commit after a partial error.

## Indexed ID Queries

`mw query ids` returns JSON only and requires exactly one of `--uid` and
`--prefix`. IDs are literal, case-sensitive strings: no trimming, slugification,
extension removal, or normalization is performed. Empty/whitespace-only exact
UIDs are rejected; whitespace inside and around nonempty IDs is preserved.
`%` and `_` are ordinary characters, not SQL wildcards. An empty prefix lists
IDs. Prefix limits default to 50 and must be 1..200; `--limit` is prefix-only.
Results use BINARY UID ordering, with case variants remaining distinct.

All successful responses have `schemaVersion: "1"`, `source: "indexed-registry"`,
and absolute `notesDir` from the existing config loader (relative paths are joined
to cwd, not canonicalized, and need not exist). Prefix responses have exactly
`mode`, `prefix`, `items`, and `hasMore` in addition to those common fields:

```json
{"schemaVersion":"1","source":"indexed-registry","notesDir":"/absolute/notes","mode":"prefix","prefix":"dev/","items":[{"uid":"dev/rust","noteID":12,"path":"dev/rust.md","title":"Rust","isHub":false}],"hasMore":false}
```

Only `limit + 1` eligible rows are fetched to determine `hasMore`; there are no
scopes, cursors, conflict inclusion options, content, or AST payloads. Persisted
conflicts of every reason, duplicate UID owners, and inconsistent note/path joins
are excluded before limiting, not resolved by choosing an arbitrary owner.

Exact responses have `mode: "exact"`, the requested literal `uid`, and one of:

- `status: "resolved"` with `target` (the same six-field object as prefix items).
- `status: "notFound"` with neither `target` nor `candidates`.
- `status: "conflict"` with the full, unbounded `candidates` group and no `target`.

Candidates contain exactly `noteID` (positive integer or null), `path` (nonempty),
`title` (string or null), `isHub` (boolean), and `reason` (nonempty). They are sorted
and deduplicated by path/reason/ID. A successful registry row alongside persisted
conflicts is included as `REGISTERED_WITH_CONFLICT`; duplicate owners produce
`DUPLICATE_ID`. Dangling note references or mismatched paths produce
`INDEX_INCONSISTENT` (or an operational error if a valid candidate cannot be
represented), never a resolved target or false not-found result. Target titles
may be empty; metadata strings are preserved.

This endpoint opens only an existing SQLite database with actual read-only flags,
checks required notes/registry columns without applying schema, and uses a
consistent read transaction per request. Missing/incompatible indexes fail
nonzero with an actionable error and no JSON fallback. Explicitly initialize,
ingest, and register separately when needed. It does not create parent directories,
perform logical database writes or migrations, scan/read Markdown, refresh caches,
run ingest/register/seal, or resolve authentication tokens. Other query contracts
are unchanged.

Live committed WAL data remains visible; `immutable=1` is not used. SQLite may
access/create WAL shared-memory sidecars and participate in locking depending on
the database mode and filesystem permissions. Read-only here is not a promise
that SQLite performs no filesystem-level sidecar or lock activity.

## Install from source

For the current Rust task workflow, install the release from this checkout:

```sh
cargo install --path crates/mw --root /Users/jdawson/.local --force --locked --target-dir target
```

This installed `/Users/jdawson/.local/bin/mw` for the 2026-10-30 cutover. Check task
capabilities, not just a generic version string. The Go instructions below are
legacy and are not a compatible fallback for migrated task sources.

Legacy Homebrew installation:

```bash
brew tap Noswad123/jamal-arcana
brew install Noswad123/jamal-arcana/mw
```

From source:

```bash
go install github.com/Noswad123/mind-weaver/cmd/mw@latest
```

For local development from a checkout:

```bash
go build -o ./bin/mw ./cmd/mw
./bin/mw --help
```

If `mw query help` does not show newer commands, check which binary is active:

```bash
which mw
mw query help
```

## Sync command boundary

Hive Sync is optional, experimental, and currently parked while the Rust port
focuses on remaining non-Hive Go features. The local notes CLI does not require
a sync API, PWA, or cloud deployment.

Top-level `mw sync` is not supported by the Rust CLI. Use `mw notes sync` or the
top-level `mw seal` shortcut for the local notes pipeline.

Current boundary:

- Rust `mw` does not expose `mw sync` commands.
- Hive Sync can evolve later as part of the separate Hive Mind app suite.

Older Hive Sync docs may still mention `mw sync`; treat those as parked legacy
notes, not as supported Rust CLI behavior.

## Querying by domain

Use `mw query notes --domain <domain>` to filter notes by frontmatter domains.

Examples:

```bash
mw query notes --domain glossary
mw query notes --domain abbreviation-index
mw query notes --domain vocabulary-index
mw query domains
```

Projection commands use `--scope` for domain intersections.

## Querying projections

Projections are structured views extracted from markdown notes into SQLite and
returned as JSON. Markdown remains the source of truth.

Current projection commands:

```bash
mw query todos
mw query recipes
mw query projection recipe
mw query ingredients
mw query ingredients --mentions
mw query ingredients --unresolved
```

`mw query projection recipe` returns a JSON array. Count records with:

```bash
mw query projection recipe | jq 'length'
```

### Projection scope

`--scope` filters a projection to source notes containing all requested domains.
It may be repeated or comma-separated:

```bash
mw query projection recipe --scope domain-a
mw query projection recipe --scope domain-a,domain-b
mw query projection recipe --scope domain-a --scope domain-b
```

For current recipe notes, no scope is needed because `recipe` means culinary
recipe:

```yaml
domains: [recipe]
```

## Glossary category filtering

`glossary` supports category filtering:

```bash
mw query notes --domain glossary --category ai
```

Category is derived from the immediate parent folder of the note path.

Example:
- `autodactyl/computer-science/software/ai/glossary.md` => category `ai`

## Notes on domain schemas

The Rust engine embeds the domain documents from
`legacy/go/internal/schema/templates/*.yaml`. Discover them without configuration:

```bash
mw schema list --json
mw schema show task --json
mw schema show devTool --json
```

Both commands always emit JSON, including when `--json` is omitted. Successful
stdout contains only one JSON object; stderr is empty. The list envelope is
`{"schemaVersion":"1","schemas":[{"domain":"abbreviation-index","aliases":["abbreviationIndex"]},...]}`.
It contains exactly ten descriptors in canonical-domain order:

| Domain | Alias |
| --- | --- |
| abbreviation-index | abbreviationIndex |
| dev-tool | devTool |
| glossary | none |
| programming-concept | programmingConcept |
| programming-implementation | programmingImplementation |
| programming-language | programmingLanguage |
| recipe | none |
| task | none |
| vocabulary-index | vocabularyIndex |
| writing-character | writingCharacter |

Domains without aliases have `aliases: []`. Show returns exactly
`{"schemaVersion":"1","domain":"task","schema":{...}}`, with the canonical
domain and the full embedded YAML semantic document as `schema`. Numeric schema
versions, `meta.allowed`, `naming`, `notes`, and nested `child_rules.fields` are
preserved; YAML comments and formatting are not part of the semantic document.

Lookup shares validation's case-sensitive canonical and lowerCamel names, trims
surrounding whitespace, and normalizes underscore/space-separated names such as
`dev_tool` or `dev tool`. Retired `task-index`/`taskIndex` names are rejected with
migration guidance, not exposed as task aliases. Unknown names, incorrect case,
filenames such as `task.yaml`, and paths are rejected with a nonzero exit and empty stdout.

These discovery commands never load config or dotenv, open databases, scan notes,
execute token commands, read external schema paths, or create directories. Config,
notes, database, and schema-path environment settings do not affect their output.

Validate notes against the embedded schemas via:

```bash
mw notes validate --domain <domain>
```

Discovery exposes the complete documents, not a promise that every declared rule
is enforced. The validator currently checks a subset of metadata and section
rules, including immediate children one heading level below their parent. It
does not enforce nested field types/object shapes or all naming/allowed-field
rules. Existing code-block parsing behavior is unchanged.

See [projections.md](projections.md) for the domain/projection model and future direction.

## Validation modes

- `mw notes validate --all`
  - File-system validation (duplicate IDs, missing hub IDs) using a fresh scan of note files.
- `mw notes validate-registry`
  - DB-backed registry conflict validation after registration.
- `mw notes issues`
  - Build/cache a clear note issue list for manual repair.

### `mw notes issues` flags

- `--json`: print structured payload (useful for editor integrations).
- `--cached`: use previously cached payload without rescanning.
- `--all`: include warnings (for example, `NOTE_NOT_IN_DB`) in addition to blocking errors.

Cache output location:

- `<NOTES_DIR>/.mw/cache/notes-issues.json`

Examples:

```bash
mw notes issues
mw notes issues --cached
mw notes issues --json
mw notes issues --all
```
