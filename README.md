# MindWeaver

![Mind Weaver](img/mind-weaver.png)

## What is MindWeaver?
MindWeaver is a local-first notes system that:
- Syncs structured notes into a SQLite database
- Enables querying and tagging
- Visualizes relationships between notes (not implemented yet)

## CLI Overview

Install from source with:

```bash
go install github.com/Noswad123/mind-weaver/cmd/mw@latest
```

```bash For full CLI details
mw -help
```

See also: `docs/cli.md`

### Core Workflows

```bash Initialize a local config
mw init --notes-dir ~/Notes
mw doctor
mw config show
```

``` bash Ingest and register notes into the database
mw notes sync
```

```bash Query notes
mw query notes --uid dataclass
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
```

```bash Watch Mode
mw notes watch
```

```bash Archive completed todos to life-log
mw todos archive
```

By default, todo archive writes to:

- `<NOTES_DIR>/introspection/life-log`

Optional override:

- `MW_LIFE_LOG_DIR` (absolute path or notes-root-relative path)

```bash Visualize Graph
mw notes graph
```

## Domain note types

MindWeaver supports multiple note shapes through domain schemas in
`internal/schema/templates/*.yaml`.

- `glossary`: richer definition-oriented notes with deeper explanation and links.
- `abbreviation-index`: scan-first abbreviation/acronym lists, usually grouped by letter.
- `vocabulary-index`: language-learning vocabulary/phrase banks, usually grouped by topic or letter.

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
mw notes fix

mw notes validate --domain glossary
mw notes validate --domain abbreviation-index
mw notes validate --domain vocabulary-index
```

### Conflict triage workflow

Use `mw notes fix` to triage blocking note ID conflicts quickly.

```bash
# scan current conflicts, cache results, fuzzy-pick files, open in nvim quickfix
mw notes fix

# print JSON payload for editor automation/integration
mw notes fix --json --no-open

# reuse the last cached conflict set
mw notes fix --cached
```

`mw notes fix` cache path:

- `<NOTES_DIR>/.mw/cache/notes-fix.json`

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

Hive Mind is bundled with this repository as an optional, experimental sync and
mobile/web companion for MindWeaver.

- `cmd/hiveSyncAPI`: optional sync API server.
- `apps/hive-pwa`: optional PWA client.
- `mw sync`: optional local sync client commands.

You do not need Hive Mind, Cloud Run, Firebase, or Postgres to use the local
`mw` notes CLI.

## Hive Sync Progress

Current implementation status and next steps for Hive Mind sync work are tracked in:

- `docs/hive-sync-progress.md`

## License

MIT. See `LICENSE`.

## Phase 2 PWA Scaffold

Initial mobile PWA scaffolding now lives in:

- `apps/hive-pwa/`
