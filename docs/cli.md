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
- `mw todos sync`
- `mw todos toggle --id <todo-id>`
- `mw todos inspect --id <todo-id>`
- `mw todos update --id <todo-id> --priority p1 --due YYYY-MM-DD`
- `mw todos archive`
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

Homebrew is preferred for normal use:

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
mw schema show task-index --json
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
| task-index | taskIndex |
| vocabulary-index | vocabularyIndex |
| writing-character | writingCharacter |

Domains without aliases have `aliases: []`. Show returns exactly
`{"schemaVersion":"1","domain":"task-index","schema":{...}}`, with the canonical
domain and the full embedded YAML semantic document as `schema`. Numeric schema
versions, `meta.allowed`, `naming`, `notes`, and nested `child_rules.fields` are
preserved; YAML comments and formatting are not part of the semantic document.

Lookup shares validation's case-sensitive canonical and lowerCamel names, trims
surrounding whitespace, and normalizes underscore/space-separated names such as
`task_index` or `task index`. Unknown names, incorrect case, filenames such as
`task-index.yaml`, and paths are rejected with a nonzero exit and empty stdout.

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

See `docs/projections.md` for the domain/projection model and future direction.

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
