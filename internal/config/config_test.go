package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWithoutConfigFile(t *testing.T) {
	home := t.TempDir()
	setConfigEnv(t, home)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantConfig := filepath.Join(home, ".config", appDirName, "config.toml")
	if cfg.ConfigPath != wantConfig {
		t.Fatalf("ConfigPath = %q, want %q", cfg.ConfigPath, wantConfig)
	}
	if cfg.NotesDir != filepath.Join(home, "Notes") {
		t.Fatalf("NotesDir = %q", cfg.NotesDir)
	}
	if cfg.DBPath != filepath.Join(home, ".local", "share", appDirName, "mind-weaver.db") {
		t.Fatalf("DBPath = %q", cfg.DBPath)
	}
	if cfg.NotesSchemaPath == "" {
		t.Fatalf("NotesSchemaPath should use embedded default")
	}
}

func TestInitWritesConfigThatCanLoad(t *testing.T) {
	home := t.TempDir()
	setConfigEnv(t, home)

	notesDir := filepath.Join(home, "my-notes")
	cfg, err := Init(InitOptions{NotesDir: notesDir})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if _, err := os.Stat(cfg.ConfigPath); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}
	if _, err := os.Stat(notesDir); err != nil {
		t.Fatalf("notes dir was not created: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after Init() error = %v", err)
	}
	if loaded.NotesDir != notesDir {
		t.Fatalf("loaded NotesDir = %q, want %q", loaded.NotesDir, notesDir)
	}
	if loaded.InboxPath != filepath.Join(notesDir, "inbox.md") {
		t.Fatalf("loaded InboxPath = %q", loaded.InboxPath)
	}
	if loaded.DashboardPath != filepath.Join(notesDir, "dashboard.md") {
		t.Fatalf("loaded DashboardPath = %q", loaded.DashboardPath)
	}
}

func setConfigEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("MW_CONFIG", "")
	t.Setenv("NOTES_DIR", "")
	t.Setenv("NOTES_DB_PATH", "")
	t.Setenv("COMMANDS_DB_PATH", "")
	t.Setenv("INBOX_PATH", "")
	t.Setenv("MW_INBOX_PATH", "")
	t.Setenv("DASHBOARD_PATH", "")
	t.Setenv("NOTES_SCHEMA_PATH", "")
	t.Setenv("SCHEMA_PATH", "")
}
