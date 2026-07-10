# MindWeaver Rust Port Checklist

Rust is the new primary implementation. The prior Go implementation remains
buildable under `legacy/go` and should be used as the fallback/oracle while the
port is incomplete.

## Current status

- [x] Move Go implementation to `legacy/go`
- [x] Create root Rust workspace
- [x] Add initial `mw` Rust CLI binary
- [x] Add initial `mw-tui` ratatui shell
- [x] Port config loading and path resolution
- [x] Port `mw init`
- [x] Port `mw doctor`
- [x] Port `mw config path/show`
- [ ] Port SQLite schema initialization
- [ ] Port note metadata/frontmatter parsing
- [ ] Port note link parsing
- [ ] Port note ingestion
- [ ] Port note registry and conflict detection
- [ ] Port `mw query notes`
- [ ] Port `mw query domains`
- [ ] Port `mw query todos`
- [ ] Port todo dashboard parsing/writeback
- [ ] Port todo archive behavior
- [ ] Port recipe projection extraction
- [ ] Port graph queries
- [ ] Port graph ratatui browser
- [ ] Port notes/todos ratatui workspace
- [ ] Port sync outbox/client
- [ ] Port Hive Sync API or decide to keep it legacy-only

## Porting rules

- Keep `legacy/go` buildable until Rust fully replaces it.
- Prefer command-by-command parity over large unverified rewrites.
- Use the Go implementation as the behavioral oracle when semantics are unclear.
- Preserve Markdown as source of truth and SQLite as derived projection/cache.
- Before changing parser/writeback behavior, add fixtures or parity checks.

## Validation commands

```bash
cargo check --workspace

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
