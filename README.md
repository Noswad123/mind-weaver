# MindWeaver

![Mind Weaver](img/mind-weaver.png)

## What is MindWeaver?
MindWeaver is a local-first notes system that:
- Syncs structured notes into a SQLite database
- Enables querying and tagging
- Visualizes relationships between notes (not implemented yet)

## CLI Overview

MindWeaver is currently being ported from Go to Rust. The root workspace now
contains the Rust implementation, while the previous Go implementation remains
available under `legacy/go` as a buildable fallback.

Build/check the Rust workspace:

```bash
cargo check --workspace
cargo run -p mw -- version
cargo run -p mw -- version --short
cargo run -p mw -- init --notes-dir ~/Notes
cargo run -p mw -- doctor
cargo run -p mw -- config show
cargo run -p mw -- db init
cargo run -p mw -- db check
cargo run -p mw -- notes sync
cargo run -p mw -- notes format --all
cargo run -p mw -- notes ingest --prune
cargo run -p mw -- notes register
cargo run -p mw -- notes validate
cargo run -p mw -- notes validate-registry
cargo run -p mw -- notes get --search dataclass
cargo run -p mw -- query notes
cargo run -p mw -- query registry
cargo run -p mw -- query domains
cargo run -p mw -- query todos
cargo run -p mw -- todos sync
cargo run -p mw -- todos toggle --id '<todo-id>'
cargo run -p mw -- todos inspect --id '<todo-id>'
cargo run -p mw -- todos update --id '<todo-id>' --priority p1 --due 2026-08-01
cargo run -p mw -- todos archive
cargo run -p mw -- query recipes
cargo run -p mw -- query ingredients
cargo run -p mw -- query graph --search dataclass --depth 1
cargo run -p mw -- notes graph --search dataclass
cargo run -p mw -- tui notes
cargo run -p mw -- tui todos
cargo run -p mw -- tui
```

Run the legacy Go CLI while porting:

```bash
cd legacy/go
go run ./cmd/mw --help
```

See `PORTING_CHECKLIST.md` for the active migration checklist.

Rust `mw notes sync` runs the local notes pipeline: format, ingest, register,
and registry validation.

Running `mw` with no subcommand launches the TUI. Use `mw tui notes` or
`mw tui todos` to open a specific workspace tab.

### Legacy Go install

Install with Homebrew:

```bash
brew tap Noswad123/jamal-arcana
brew install Noswad123/jamal-arcana/mw
```

Or install from source with:

```bash
go install github.com/Noswad123/mind-weaver/cmd/mw@latest
```

For local development from a checkout:

```bash
go build -o ./bin/mw ./cmd/mw
./bin/mw --help
```

If newer query commands are missing, verify that your shell is using the updated
binary:

```bash
which mw
mw query help
```

```bash For full CLI details
mw -help
```

See also: `docs/cli.md`

For the current domain/projection architecture and future direction, see
`docs/projections.md`.

### Core Workflows

```bash Initialize a local config
mw init --notes-dir ~/Notes
mw doctor
mw config show
```

``` bash Ingest and register notes into the database
mw notes sync
# shortcut: mw seal
# nested alias: mw notes seal
```

```bash Query notes
mw query notes --uid dataclass
```

```bash Query available domains and projections
mw query domains
mw query todos
mw query projection recipe
mw query ingredients
```

```bash Query glossary notes by category
mw query notes --domain glossary --category biology
```

```bash Query abbreviation index notes
mw query notes --domain abbreviation-index
```

```bash Query vocabulary index notes
mw query notes --domain vocabulary-index
```

```bash Retrieve Notes
mw notes get --search "dataclass"
# shortcut: mw get --search "dataclass" or mw summon --search "dataclass"
```

```bash Watch Mode
mw notes watch
mw notes watch --fg
mw notes watch --status
mw notes watch --stop
```

Watch mode runs in the background by default. `--fg` stops any background watcher
and runs the watcher in the foreground. It polls Markdown files and refreshes the
local projection after changes with `ingest --prune → register → validate-registry`.
Add `--format` if you want the watcher to run formatting as part of that refresh
pipeline.

```bash Archive completed todos to life-log
mw todos archive
```

By default, todo archive writes to:

- `<NOTES_DIR>/introspection/life-log`

Optional override:

- `MW_LIFE_LOG_DIR` (absolute path or notes-root-relative path)

```bash Visualize Graph
mw notes graph
# shortcut: mw graph or mw loom
```

Note workflow shortcuts:

- `mw seal` → `mw notes sync`
- `mw get` / `mw summon` → `mw notes get`
- `mw ingest` / `mw banish` → `mw notes ingest`
- `mw format` / `mw meld` → `mw notes format`
- `mw register`, `mw validate`, `mw validate-registry`, `mw graph` / `mw loom`

Top-level `mw sync` is not supported by the Rust CLI. Use `mw seal` for the
local notes pipeline; Hive Sync remains parked as optional app-suite work.

## Domain note types

MindWeaver supports multiple note shapes through domain schemas in
`internal/schema/templates/*.yaml`.

- `glossary`: richer definition-oriented notes with deeper explanation and links.
- `abbreviation-index`: scan-first abbreviation/acronym lists, usually grouped by letter.
- `vocabulary-index`: language-learning vocabulary/phrase banks, usually grouped by topic or letter.
- `recipe`: culinary recipe notes with extracted recipe/ingredient projections.

Domains can also be composed. Prefer structural domains for shape and scope
domains only where they add useful distinction:

```yaml
domains: [recipe]
domains: [glossary, aviation]
domains: [vocabulary, japanese]
domains: [protocol, biology, mrna]
```

### Projections

Projections are structured SQLite-backed views extracted from Markdown notes and
exposed as JSON commands. Markdown remains the source of truth; SQLite is the
derived projection/cache.

Current projection-oriented commands include:

```bash
mw query todos
mw query recipes
mw query projection recipe
mw query ingredients
mw query ingredients --mentions
```

Projection scope filtering accepts repeated or comma-separated domains:

```bash
mw query projection recipe --scope some-domain
mw query projection recipe --scope domain-a,domain-b
mw query projection recipe --scope domain-a --scope domain-b
```

For current purposes, `recipe` means culinary recipe, so recipe notes use
`domains: [recipe]` and do not need a redundant `cooking` scope domain.

### Glossary category behavior

For `glossary`, the category is derived from the immediate parent folder.

- `autodactyl/computer-science/software/ai/glossary.md` => category `ai`

Use category filtering with:

```bash
mw query notes --domain glossary --category ai
```

### Domain validation

```bash
mw notes validate --all
mw notes validate-registry
mw notes issues

mw notes validate --domain glossary
mw notes validate --domain abbreviation-index
mw notes validate --domain vocabulary-index
```

### Conflict triage workflow

Use `mw notes issues` to triage blocking note ID conflicts and registry issues
quickly. The legacy `mw notes fix` fuzzy/editor workflow is intentionally not
part of the Rust CLI.

```bash
# scan current issues and cache results
mw notes issues

# print JSON payload for editor automation/integration
mw notes issues --json

# reuse the last cached conflict set
mw notes issues --cached
```

`mw notes issues` cache path:

- `<NOTES_DIR>/.mw/cache/notes-issues.json`

## Configuration

MindWeaver uses XDG config/data paths by default:

- `${XDG_CONFIG_HOME:-~/.config}/mind-weaver/config.toml`
- `${XDG_DATA_HOME:-~/.local/share}/mind-weaver/mind-weaver.db`

SQLite schemas are embedded in the binary, so a released `mw` does not need a
checked-out repository or local `db/schema.sql` file to initialize its database.

Create a starter config with:

```bash
mw init --notes-dir ~/Notes
```

Compatibility environment overrides are still supported:

- `NOTES_DIR`
- `NOTES_DB_PATH`
- `COMMANDS_DB_PATH`
- `INBOX_PATH` or `MW_INBOX_PATH`
- `SCHEMA_PATH` or `NOTES_SCHEMA_PATH` for development schema overrides
- `DASHBOARD_PATH`

Neorg integration is no longer supported; MindWeaver targets Markdown notes.

Optional Hive Mind settings can also live in `config.toml`:

```toml
[hive_sync]
enabled = false
endpoint = "http://127.0.0.1:8080"
device_id = ""
device_name = ""
app_version = "mw-dev"
token_from_keychain = false
token_keychain_service = "mw/hive-sync"
conflicts_dir = "~/.local/share/mind-weaver/conflicts"

[hive_pwa]
enabled = false
url = ""
api_url = ""
```

CLI flags and `HIVE_SYNC_*` / `HIVE_PWA_*` environment variables still override
config values for one-off runs.

## Optional Hive Mind components

The Hive Mind implementation is still bundled with this repository as optional,
experimental sync/API/mobile code around MindWeaver. Its docs now live in the
separate `/Users/jdawson/Projects/hive-mind` project. Hive should be treated as
a separate companion, not as required functionality for the local Markdown notes
CLI.

Hive Sync is currently parked while the Rust port focuses on remaining non-Hive
Go features.

- `legacy/go/cmd/hiveSyncAPI`: optional sync API server.
- `apps/hive-pwa`: optional PWA client.
- Long-term command/binary name: `hive`.
- Rust `mw` does not expose top-level `mw sync`; local notes use `mw notes sync`
  or `mw seal` instead. Hive Sync is parked while the Rust port remains focused
  on local notes workflows.

You do not need Hive Mind, Cloud Run, Firebase, or Postgres to use the local
`mw` notes CLI.

The current Hive capability boundary and resume notes now live in the separate
local project:

- `/Users/jdawson/Projects/hive-mind/plan.md`

## Hive Sync Progress

Current implementation status and next steps for Hive Mind sync work are tracked in:

- `/Users/jdawson/Projects/hive-mind/docs/hive-sync-progress.md`

## License

MIT. See `LICENSE`.

## Phase 2 PWA Scaffold

Initial mobile PWA scaffolding now lives in:

- `apps/hive-pwa/`
