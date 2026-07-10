# MindWeaver Rust Port Checklist

Rust is the new primary implementation. The prior Go implementation remains
buildable under `legacy/go` and should be used as the fallback/oracle while the
port is incomplete.

Hive Mind / Hive Sync is now parked as a separate optional app suite around
MindWeaver, not as mandatory Rust CLI parity. The Rust `mw` port owns the local
Markdown-first notes workflow; Hive owns optional sync/API/PWA behavior. Do not
spend more porting time on Hive right now beyond keeping the current notes in
`hive-mind-plan.md` useful for later.

## Current status

- [x] Move Go implementation to `legacy/go`
- [x] Create root Rust workspace
- [x] Add initial `mw` Rust CLI binary
- [x] Add initial `mw-tui` ratatui shell
- [x] Port config loading and path resolution
- [x] Port `mw init`
- [x] Port `mw doctor`
- [x] Port `mw config path/show`
- [x] Port SQLite schema initialization
- [x] Port note metadata/frontmatter parsing
- [x] Port note link parsing
- [x] Port note ingestion
- [x] Port note registry and conflict detection
- [x] Port `mw query notes`
- [x] Port `mw query domains`
- [x] Port `mw query todos`
- [x] Port todo dashboard parsing/writeback
- [x] Port todo archive behavior
- [x] Port recipe projection extraction
- [x] Port graph queries
- [x] Port graph ratatui browser
- [x] Port notes/todos ratatui workspace
- [x] Port `mw notes format`
- [x] Port `mw notes sync` pipeline (`format → ingest → register → validate-registry`)
- [x] Port `mw notes validate` filesystem registry checks
- [x] Port `mw notes validate-registry`
- [x] Port `mw query registry`
- [x] Port `mw todos inspect`
- [x] Port `mw todos toggle`
- [x] Port `mw todos update`
- [x] Port `mw notes get` / `summon`
- [x] Port `mw notes ingest --prune`
- [x] Add `mw tui notes` / `mw tui todos` entrypoints and keep bare `mw` launching TUI
- [x] Remove Rust top-level `mw sync` support; keep Hive Sync parked outside the local notes CLI
- [x] Decide Hive Sync API can remain in separate Hive app suite
- [x] Add local CLI parity polish: nested note aliases and `mw version --short`

## Active non-Hive port backlog

Focus here next. Use `legacy/go` as the oracle for behavior and CLI shape.

- [x] Port `mw notes get` / `summon`
- [x] Port `mw notes sync` pipeline (`format → ingest → register → validate-registry`)
- [x] Port `mw notes ingest --prune`
- [x] Port `mw notes format`
- [x] Port `mw notes validate` filesystem registry checks
- [x] Port `mw notes validate-registry`
- [x] Port `mw notes validate --domain`
- [x] Replace legacy `mw notes fix` with Rust-native `mw notes issues` workflow
- [x] Port `mw notes watch` background default plus `--fg` / `--status` / `--stop` / `--restart`
- [x] Port note subcommand shortcuts where still useful:
  - `mw get` / `mw summon` → `mw notes get`
  - `mw seal` → `mw notes sync` (Rust `mw sync` is unsupported)
  - `mw ingest` / `mw banish` → `mw notes ingest`
  - `mw format` / `mw meld` → `mw notes format`
  - `mw register`, `mw validate`, `mw validate-registry`, `mw graph` / `mw loom`
- [x] Add nested note aliases (`mw notes seal|banish|meld|loom|validate-db`) and `mw version --short`
- [x] Port `mw query registry`
- [x] Port `mw todos toggle`
- [x] Port `mw todos inspect`
- [x] Port `mw todos update`
- [x] Keep bare `mw` / `mw tui todos` as replacement path for interactive todo dashboard

## Deferred Hive parking lot

Do not prioritize these until the non-Hive Go feature port is substantially
complete or there is a concrete need for mobile/cloud sync work.

- [ ] Define stable Hive sync protocol contract
- [ ] Decide long-term desktop sync binary boundary (`legacy/go mw sync`, `hive`, or `mw-hive`)
- [ ] Decide whether to extract Hive into its own repository/app packaging boundary
- [ ] Reintroduce sync under a non-conflicting Hive boundary if/when Hive work resumes
- [ ] Run physical-device PWA smoke tests and capture evidence

## Porting rules

- Keep `legacy/go` buildable until Rust fully replaces it.
- Prefer command-by-command parity over large unverified rewrites.
- Use the Go implementation as the behavioral oracle when semantics are unclear.
- Preserve Markdown as source of truth and SQLite as derived projection/cache.
- Before changing parser/writeback behavior, add fixtures or parity checks.
- Do not block the Rust `mw` local notes port on full Hive Sync HTTP push/pull;
  keep Hive as an optional app boundary unless that decision changes.
- Prefer finishing non-Hive Go feature parity before returning to Hive.

## Validation commands

```bash
cargo check --workspace
cargo test --workspace
cargo run -p mw -- db init
cargo run -p mw -- db check

(cd legacy/go && go test ./...)
(cd legacy/go && go build -o ./bin/mw ./cmd/mw)
(cd legacy/go && ./scripts/smoke-fresh-install.sh)
```

## Useful fallback commands

```bash
cd legacy/go
go run ./cmd/mw --help
go run ./cmd/mw notes sync
go run ./cmd/mw query todos
```
