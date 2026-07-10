#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="${TMPDIR:-/tmp}/mind-weaver-smoke-$$"
BIN_PATH="$TMP_DIR/bin/mw"
NOTES_DIR="$TMP_DIR/notes"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

mkdir -p "$TMP_DIR/bin"

go build -o "$BIN_PATH" "$ROOT_DIR/cmd/mw"

export HOME="$TMP_DIR/home"
export XDG_CONFIG_HOME="$TMP_DIR/config"
export XDG_DATA_HOME="$TMP_DIR/data"
export XDG_STATE_HOME="$TMP_DIR/state"
export NOTES_DIR=
export NOTES_DB_PATH=
export COMMANDS_DB_PATH=
export SCHEMA_PATH=
export NOTES_SCHEMA_PATH=
export DASHBOARD_PATH=
export INBOX_PATH=
export MW_INBOX_PATH=

"$BIN_PATH" --help >/dev/null
"$BIN_PATH" init --notes-dir "$NOTES_DIR" --force >/dev/null
"$BIN_PATH" config path >/dev/null
"$BIN_PATH" config show >/dev/null
"$BIN_PATH" doctor >/dev/null
"$BIN_PATH" notes ingest >/dev/null

test -f "$XDG_CONFIG_HOME/mind-weaver/config.toml"
test -f "$XDG_DATA_HOME/mind-weaver/mind-weaver.db"

echo "✅ fresh-install smoke passed"
