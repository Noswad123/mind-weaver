# CLI Reference (Quick)

## Core commands

- `mw -help`
- `mw init --notes-dir <path>`
- `mw doctor`
- `mw config path`
- `mw config show`
- `mw notes sync`
- `mw notes format --all`
- `mw notes ingest`
- `mw notes get --search <query>`
- `mw notes validate --all`
- `mw notes validate-registry`
- `mw notes fix`
- `mw notes validate --domain <domain>`
- `mw query notes --uid <uid>`
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

`mw` embeds its default SQLite schemas; `SCHEMA_PATH` / `NOTES_SCHEMA_PATH` are
only needed when intentionally testing an alternate schema file.

Rust note validation currently covers filesystem/registry ID checks and
DB-backed registry conflicts. Domain-schema validation remains a legacy Go
fallback until ported.

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

## Sync commands (Hive Sync)

Hive Sync is optional, experimental, and currently parked while the Rust port
focuses on remaining non-Hive Go features. The local notes CLI does not require
a sync API, PWA, or cloud deployment.

Current boundary:

- Rust `mw` supports local sync outbox/diagnostics/dry-run commands.
- Full HTTP push/pull sync remains in the legacy Go sync client and can evolve as
  part of the separate Hive Mind app suite.

Rust-local examples:

- `mw sync doctor --skip-remote`
- `mw sync outbox --format text`
- `mw sync run --dry-run`

Legacy/full Hive Sync examples:

- `mw sync --endpoint <url> --device-id <id> --token <token>`
- `mw sync doctor --endpoint <url> [--token <token>]`
- `mw sync conflicts review [--older-than 168h] [--export-dir <dir>] [--mark-resolved]`
- `mw sync token store --token-stdin --device-id <id>` (macOS Keychain)
- `mw sync token check --endpoint <url> --device-id <id> [token options]`

Token input options for `mw sync` and `mw sync doctor`:

- `--token <value>`
- `HIVE_SYNC_TOKEN`
- `--token-command '<command that prints token>'`
- `--token-from-keychain` (macOS)

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
- `mw notes fix`
  - Build/cache conflict list, fuzzy-pick problematic files, and open selected files in Neovim quickfix.

### `mw notes fix` flags

- `--json`: print structured payload (useful for editor integrations).
- `--cached`: use previously cached payload without rescanning.
- `--all`: include warnings (for example, `NOTE_NOT_IN_DB`) in addition to blocking errors.
- `--no-open`: do not launch Neovim; print selected conflicts.
- `--no-fuzzy`: skip fuzzy picker and include all collected conflicts.

Cache output location:

- `<NOTES_DIR>/.mw/cache/notes-fix.json`

Examples:

```bash
mw notes fix
mw notes fix --cached
mw notes fix --json --no-open
mw notes fix --all --no-fuzzy --no-open
```
