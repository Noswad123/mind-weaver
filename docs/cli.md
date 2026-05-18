# CLI Reference (Quick)

## Core commands

- `mw -help`
- `mw init --notes-dir <path>`
- `mw doctor`
- `mw config path`
- `mw config show`
- `mw notes sync`
- `mw notes ingest`
- `mw notes get --search <query>`
- `mw notes validate --all`
- `mw notes validate-registry`
- `mw notes fix`
- `mw notes validate --domain <domain>`
- `mw query notes --uid <uid>`
- `mw todos sync`
- `mw todos archive`

`mw` embeds its default SQLite schemas; `SCHEMA_PATH` / `NOTES_SCHEMA_PATH` are
only needed when intentionally testing an alternate schema file.

## Install from source

```bash
go install github.com/Noswad123/mind-weaver/cmd/mw@latest
```

## Sync commands (Hive Sync)

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
- existing project domains such as `task-index`, `programming-concept`, etc.

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
