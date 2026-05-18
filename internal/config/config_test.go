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

func TestLoadHiveConfig(t *testing.T) {
	home := t.TempDir()
	setConfigEnv(t, home)
	configPath := filepath.Join(home, ".config", appDirName, "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`notes_dir = "~/Notes"

[hive_sync]
enabled = true
endpoint = "https://sync.example.com"
device_id = "laptop"
token_from_keychain = true
outbox_limit = 25
worker_interval = "30s"

[hive_pwa]
enabled = true
url = "https://pwa.example.com"
api_url = "https://sync.example.com"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.HiveSync.Enabled || cfg.HiveSync.Endpoint != "https://sync.example.com" || cfg.HiveSync.DeviceID != "laptop" {
		t.Fatalf("unexpected hive sync config: %+v", cfg.HiveSync)
	}
	if !cfg.HiveSync.TokenFromKeychain || cfg.HiveSync.OutboxLimit != 25 || cfg.HiveSync.WorkerInterval != "30s" {
		t.Fatalf("unexpected hive sync options: %+v", cfg.HiveSync)
	}
	if !cfg.HivePWA.Enabled || cfg.HivePWA.URL != "https://pwa.example.com" || cfg.HivePWA.APIURL != "https://sync.example.com" {
		t.Fatalf("unexpected hive pwa config: %+v", cfg.HivePWA)
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
	t.Setenv("HIVE_SYNC_API_URL", "")
	t.Setenv("HIVE_SYNC_DEVICE_ID", "")
	t.Setenv("HIVE_SYNC_DEVICE_NAME", "")
	t.Setenv("HIVE_SYNC_PLATFORM", "")
	t.Setenv("HIVE_SYNC_APP_VERSION", "")
	t.Setenv("HIVE_SYNC_TOKEN_COMMAND", "")
	t.Setenv("HIVE_SYNC_TOKEN_FROM_KEYCHAIN", "")
	t.Setenv("HIVE_SYNC_TOKEN_KEYCHAIN_SERVICE", "")
	t.Setenv("HIVE_SYNC_TOKEN_KEYCHAIN_ACCOUNT", "")
	t.Setenv("HIVE_SYNC_CONFLICTS_DIR", "")
	t.Setenv("HIVE_SYNC_OUTBOX_LIMIT", "")
	t.Setenv("HIVE_SYNC_PULL_LIMIT", "")
	t.Setenv("HIVE_SYNC_RETRY_ATTEMPTS", "")
	t.Setenv("HIVE_SYNC_RETRY_BASE_DELAY", "")
	t.Setenv("HIVE_SYNC_RETRY_MAX_DELAY", "")
	t.Setenv("HIVE_SYNC_WORKER_ITERATIONS", "")
	t.Setenv("HIVE_SYNC_UNTIL_EMPTY", "")
	t.Setenv("HIVE_SYNC_UNTIL_EMPTY_MAX_ITERATIONS", "")
	t.Setenv("HIVE_SYNC_WORKER_INTERVAL", "")
	t.Setenv("HIVE_PWA_ENABLED", "")
	t.Setenv("HIVE_PWA_URL", "")
	t.Setenv("VITE_HIVE_SYNC_API_URL", "")
}
