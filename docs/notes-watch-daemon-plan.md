# Notes Watch Daemon Plan

Last updated: 2026-07-10

## Intent

`mw notes watch` should become a background-friendly notes watcher rather than a
foreground-only file watch loop. The goal is to keep the SQLite projection fresh
while preserving Markdown as the source of truth.

## Desired UX

Default background mode:

```bash
mw notes watch
```

Foreground/process management:

```bash
mw notes watch --fg
mw notes watch --status
mw notes watch --stop
mw notes watch --restart
```

Possible platform integration later:

```bash
mw notes watch install-agent
mw notes watch uninstall-agent
```

On macOS, `install-agent` could install a LaunchAgent plist. On Linux, it could
emit or install a user systemd unit.

## Expected behavior

- Watch configured `notes_dir` for Markdown changes.
- Debounce bursts of filesystem events.
- Run the safe local projection pipeline after changes:
  - `mw notes format` if enabled by config/flag
  - `mw notes ingest --prune`
  - `mw notes register`
  - `mw notes validate-registry`
- Log status and errors to a predictable file under MindWeaver's data dir.
- Store PID/status metadata somewhere predictable, e.g.
  `~/.local/share/mind-weaver/watch.pid`.
- Avoid multiple watcher instances for the same notes dir.

## Safety constraints

- Do not make background formatting surprising. Consider defaulting daemon mode
  to ingest/register only unless `--format` is explicitly set.
- Never watch `.git`.
- Avoid tight loops if validation fails.
- Prefer clear logs over silent retries.
- Keep the foreground mode useful for debugging before adding service managers.

## Implementation notes for later

Rust crate candidates:

- `notify` for filesystem events.
- `daemonize` or a small platform-specific fork/session approach if true daemon
  mode is needed.
- For a simpler first pass, default `mw notes watch` can spawn an internal
  foreground worker as a detached child and write a pidfile. `--fg` can stop an
  existing background watcher and run the same loop in the current terminal.

Suggested phased approach:

1. Implement foreground watcher with debounce and projection pipeline.
2. Add status/log/pid plumbing.
3. Add background default, `--fg`, `--stop`, and `--restart` by
   spawning/stopping the foreground watcher process.
4. Add LaunchAgent/systemd helpers only after the CLI behavior is stable.

## Current implementation

Rust `mw` now includes the first stable pass:

- background process wrapper: `mw notes watch`
- foreground polling watcher: `mw notes watch --fg`
- status/stop/restart: `--status`, `--stop`, `--restart`
- optional formatting: `--format`
- state files next to the configured SQLite DB:
  - `watch.pid`
  - `watch.log`

The first pass intentionally uses polling instead of platform-specific filesystem
events so the CLI behavior is stable before adding LaunchAgent/systemd helpers.
