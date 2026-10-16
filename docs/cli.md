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

Domain schemas live in `internal/schema/templates/*.yaml` and are validated via:

```bash
mw notes validate --domain <domain>
```

Current domain templates include:
- `glossary`
- `abbreviation-index`
- `vocabulary-index`
- `recipe`
- existing project domains such as `task-index`, `programming-concept`, etc.

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
