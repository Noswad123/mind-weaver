package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/infra/db"
	"github.com/joho/godotenv"
)

const appDirName = "mind-weaver"

type Config struct {
	ConfigPath         string
	NotesDir           string
	DBPath             string
	CommandsDBPath     string
	InboxPath          string
	DashboardPath      string
	NotesSchemaPath    string
	CommandsSchemaPath string
	HiveSync           HiveSyncConfig
	HivePWA            HivePWAConfig
}

type HiveSyncConfig struct {
	Enabled                 bool
	Endpoint                string
	DeviceID                string
	DeviceName              string
	Platform                string
	AppVersion              string
	TokenCommand            string
	TokenFromKeychain       bool
	TokenKeychainService    string
	TokenKeychainAccount    string
	ConflictsDir            string
	OutboxLimit             int
	PullLimit               int
	RetryAttempts           int
	RetryBaseDelay          string
	RetryMaxDelay           string
	WorkerIterations        int
	UntilEmpty              bool
	UntilEmptyMaxIterations int
	WorkerInterval          string
}

type HivePWAConfig struct {
	Enabled bool
	URL     string
	APIURL  string
}

type InitOptions struct {
	ConfigPath string
	NotesDir   string
	DBPath     string
	InboxPath  string
	Force      bool
}

type CheckStatus string

const (
	CheckOK   CheckStatus = "ok"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
)

type Check struct {
	Name    string
	Status  CheckStatus
	Message string
}

func Default() Config {
	notesDir := ExpandPath("~/Notes")
	dataDir := DataDir()
	return Config{
		ConfigPath:         DefaultConfigPath(),
		NotesDir:           notesDir,
		DBPath:             filepath.Join(dataDir, "mind-weaver.db"),
		CommandsDBPath:     filepath.Join(dataDir, "command.db"),
		InboxPath:          filepath.Join(notesDir, "inbox.md"),
		DashboardPath:      filepath.Join(notesDir, "dashboard.md"),
		NotesSchemaPath:    db.EmbeddedSchemaPath,
		CommandsSchemaPath: db.EmbeddedCommandSchemaPath,
		HiveSync: HiveSyncConfig{
			Endpoint:                "http://127.0.0.1:8080",
			AppVersion:              "mw-dev",
			TokenKeychainService:    "mw/hive-sync",
			ConflictsDir:            filepath.Join(dataDir, "conflicts"),
			OutboxLimit:             100,
			PullLimit:               100,
			RetryAttempts:           3,
			RetryBaseDelay:          "500ms",
			RetryMaxDelay:           "5s",
			WorkerIterations:        1,
			UntilEmptyMaxIterations: 100,
			WorkerInterval:          "15s",
		},
	}
}

func Load() (Config, error) {
	cfg := Default()
	path := strings.TrimSpace(os.Getenv("MW_CONFIG"))
	if path == "" {
		path = cfg.ConfigPath
	}
	path = ExpandPath(path)
	cfg.ConfigPath = path
	if !fileExists(path) {
		loadDotEnvIfPresent()
		if envPath := strings.TrimSpace(os.Getenv("MW_CONFIG")); envPath != "" {
			path = ExpandPath(envPath)
			cfg.ConfigPath = path
		}
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := parseConfig(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}

	applyEnvOverrides(&cfg)
	normalize(&cfg)
	return cfg, nil
}

func Init(opts InitOptions) (Config, error) {
	cfg := Default()
	if strings.TrimSpace(opts.ConfigPath) != "" {
		cfg.ConfigPath = ExpandPath(opts.ConfigPath)
	}
	if strings.TrimSpace(opts.NotesDir) != "" {
		cfg.NotesDir = ExpandPath(opts.NotesDir)
		if strings.TrimSpace(opts.InboxPath) == "" {
			cfg.InboxPath = filepath.Join(cfg.NotesDir, "inbox.md")
		}
		cfg.DashboardPath = filepath.Join(cfg.NotesDir, "dashboard.md")
	}
	if strings.TrimSpace(opts.DBPath) != "" {
		cfg.DBPath = ExpandPath(opts.DBPath)
	}
	if strings.TrimSpace(opts.InboxPath) != "" {
		cfg.InboxPath = ExpandPath(opts.InboxPath)
	}
	normalize(&cfg)

	if _, err := os.Stat(cfg.ConfigPath); err == nil && !opts.Force {
		return cfg, fmt.Errorf("config already exists at %s; use --force to overwrite", cfg.ConfigPath)
	} else if err != nil && !os.IsNotExist(err) {
		return cfg, fmt.Errorf("stat config %s: %w", cfg.ConfigPath, err)
	}

	for _, dir := range []string{filepath.Dir(cfg.ConfigPath), DataDir(), StateDir(), filepath.Dir(cfg.DBPath), filepath.Dir(cfg.CommandsDBPath), cfg.NotesDir, filepath.Dir(cfg.InboxPath), filepath.Dir(cfg.DashboardPath)} {
		if strings.TrimSpace(dir) == "" || dir == "." {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return cfg, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(cfg.ConfigPath, []byte(Format(cfg)), 0o644); err != nil {
		return cfg, fmt.Errorf("write config %s: %w", cfg.ConfigPath, err)
	}

	return cfg, nil
}

func Format(cfg Config) string {
	normalize(&cfg)
	return fmt.Sprintf(`# MindWeaver configuration
notes_dir = %q
db_path = %q
commands_db_path = %q
inbox_path = %q
dashboard_path = %q

[hive_sync]
enabled = %t
endpoint = %q
device_id = %q
device_name = %q
app_version = %q
token_command = %q
token_from_keychain = %t
token_keychain_service = %q
token_keychain_account = %q
conflicts_dir = %q
outbox_limit = %d
pull_limit = %d
retry_attempts = %d
retry_base_delay = %q
retry_max_delay = %q
worker_iterations = %d
until_empty = %t
until_empty_max_iterations = %d
worker_interval = %q

[hive_pwa]
enabled = %t
url = %q
api_url = %q
`, CollapseHome(cfg.NotesDir), CollapseHome(cfg.DBPath), CollapseHome(cfg.CommandsDBPath), CollapseHome(cfg.InboxPath), CollapseHome(cfg.DashboardPath), cfg.HiveSync.Enabled, cfg.HiveSync.Endpoint, cfg.HiveSync.DeviceID, cfg.HiveSync.DeviceName, cfg.HiveSync.AppVersion, cfg.HiveSync.TokenCommand, cfg.HiveSync.TokenFromKeychain, cfg.HiveSync.TokenKeychainService, cfg.HiveSync.TokenKeychainAccount, CollapseHome(cfg.HiveSync.ConflictsDir), cfg.HiveSync.OutboxLimit, cfg.HiveSync.PullLimit, cfg.HiveSync.RetryAttempts, cfg.HiveSync.RetryBaseDelay, cfg.HiveSync.RetryMaxDelay, cfg.HiveSync.WorkerIterations, cfg.HiveSync.UntilEmpty, cfg.HiveSync.UntilEmptyMaxIterations, cfg.HiveSync.WorkerInterval, cfg.HivePWA.Enabled, cfg.HivePWA.URL, cfg.HivePWA.APIURL)
}

func Doctor(cfg Config, loadErr error) []Check {
	normalize(&cfg)
	checks := []Check{
		{Name: "config path", Status: CheckOK, Message: cfg.ConfigPath},
	}

	if loadErr != nil {
		checks = append(checks, Check{Name: "config load", Status: CheckFail, Message: loadErr.Error()})
	} else if fileExists(cfg.ConfigPath) {
		checks = append(checks, Check{Name: "config load", Status: CheckOK, Message: "config file loaded"})
	} else {
		checks = append(checks, Check{Name: "config load", Status: CheckWarn, Message: "config file does not exist; run `mw init`"})
	}

	checks = append(checks,
		pathExistsCheck("notes directory", cfg.NotesDir, true),
		parentCheck("inbox path", cfg.InboxPath),
		parentCheck("database path", cfg.DBPath),
		schemaCheck("notes schema", cfg.NotesSchemaPath),
		schemaCheck("commands schema", cfg.CommandsSchemaPath),
		hiveSyncCheck(cfg.HiveSync),
		hivePWACheck(cfg.HivePWA),
	)

	return checks
}

func ConfigDir() string {
	if v := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); v != "" {
		return ExpandPath(v)
	}
	return filepath.Join(homeDir(), ".config")
}

func DataDir() string {
	if v := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); v != "" {
		return filepath.Join(ExpandPath(v), appDirName)
	}
	return filepath.Join(homeDir(), ".local", "share", appDirName)
}

func StateDir() string {
	if v := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); v != "" {
		return filepath.Join(ExpandPath(v), appDirName)
	}
	return filepath.Join(homeDir(), ".local", "state", appDirName)
}

func DefaultConfigPath() string {
	return filepath.Join(ConfigDir(), appDirName, "config.toml")
}

func ExpandPath(path string) string {
	path = strings.TrimSpace(os.ExpandEnv(path))
	if path == "~" {
		return homeDir()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir(), strings.TrimPrefix(path, "~/"))
	}
	return path
}

func CollapseHome(path string) string {
	home := homeDir()
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func parseConfig(data []byte, cfg *Config) error {
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}

		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("line %d: expected key = value", lineNo)
		}
		key = strings.TrimSpace(key)
		value := strings.TrimSpace(raw)

		switch section {
		case "":
			str, err := parseStringValue(value)
			if err != nil {
				return fmt.Errorf("line %d: %w", lineNo, err)
			}
			switch key {
			case "notes_dir":
				cfg.NotesDir = str
			case "db_path":
				cfg.DBPath = str
			case "commands_db_path":
				cfg.CommandsDBPath = str
			case "inbox_path":
				cfg.InboxPath = str
			case "dashboard_path":
				cfg.DashboardPath = str
			case "notes_schema_path":
				cfg.NotesSchemaPath = str
			case "commands_schema_path":
				cfg.CommandsSchemaPath = str
			}
		case "hive_sync":
			if err := parseHiveSyncValue(key, value, cfg); err != nil {
				return fmt.Errorf("line %d: %w", lineNo, err)
			}
		case "hive_pwa":
			if err := parseHivePWAValue(key, value, cfg); err != nil {
				return fmt.Errorf("line %d: %w", lineNo, err)
			}
		}
	}
	return scanner.Err()
}

func parseStringValue(value string) (string, error) {
	if strings.HasPrefix(value, `"`) {
		return strconv.Unquote(value)
	}
	return strings.TrimSpace(value), nil
}

func stripComment(line string) string {
	inString := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if r == '#' && !inString {
			return line[:i]
		}
	}
	return line
}

func applyEnvOverrides(cfg *Config) {
	setString := func(env string, dest *string) {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			*dest = v
		}
	}
	setString("NOTES_DIR", &cfg.NotesDir)
	setString("NOTES_DB_PATH", &cfg.DBPath)
	setString("COMMANDS_DB_PATH", &cfg.CommandsDBPath)
	setString("INBOX_PATH", &cfg.InboxPath)
	setString("MW_INBOX_PATH", &cfg.InboxPath)
	setString("DASHBOARD_PATH", &cfg.DashboardPath)
	setString("HIVE_SYNC_API_URL", &cfg.HiveSync.Endpoint)
	setString("HIVE_SYNC_DEVICE_ID", &cfg.HiveSync.DeviceID)
	setString("HIVE_SYNC_DEVICE_NAME", &cfg.HiveSync.DeviceName)
	setString("HIVE_SYNC_PLATFORM", &cfg.HiveSync.Platform)
	setString("HIVE_SYNC_APP_VERSION", &cfg.HiveSync.AppVersion)
	setString("HIVE_SYNC_TOKEN_COMMAND", &cfg.HiveSync.TokenCommand)
	setString("HIVE_SYNC_TOKEN_KEYCHAIN_SERVICE", &cfg.HiveSync.TokenKeychainService)
	setString("HIVE_SYNC_TOKEN_KEYCHAIN_ACCOUNT", &cfg.HiveSync.TokenKeychainAccount)
	setString("HIVE_SYNC_CONFLICTS_DIR", &cfg.HiveSync.ConflictsDir)
	setString("HIVE_PWA_URL", &cfg.HivePWA.URL)
	setString("VITE_HIVE_SYNC_API_URL", &cfg.HivePWA.APIURL)

	setBool := func(env string, dest *bool) {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			if parsed, err := strconv.ParseBool(v); err == nil {
				*dest = parsed
			}
		}
	}
	setBool("HIVE_SYNC_ENABLED", &cfg.HiveSync.Enabled)
	setBool("HIVE_SYNC_TOKEN_FROM_KEYCHAIN", &cfg.HiveSync.TokenFromKeychain)
	setBool("HIVE_SYNC_UNTIL_EMPTY", &cfg.HiveSync.UntilEmpty)
	setBool("HIVE_PWA_ENABLED", &cfg.HivePWA.Enabled)

	setInt := func(env string, dest *int) {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				*dest = parsed
			}
		}
	}
	setInt("HIVE_SYNC_OUTBOX_LIMIT", &cfg.HiveSync.OutboxLimit)
	setInt("HIVE_SYNC_PULL_LIMIT", &cfg.HiveSync.PullLimit)
	setInt("HIVE_SYNC_RETRY_ATTEMPTS", &cfg.HiveSync.RetryAttempts)
	setInt("HIVE_SYNC_WORKER_ITERATIONS", &cfg.HiveSync.WorkerIterations)
	setInt("HIVE_SYNC_UNTIL_EMPTY_MAX_ITERATIONS", &cfg.HiveSync.UntilEmptyMaxIterations)

	setString("HIVE_SYNC_RETRY_BASE_DELAY", &cfg.HiveSync.RetryBaseDelay)
	setString("HIVE_SYNC_RETRY_MAX_DELAY", &cfg.HiveSync.RetryMaxDelay)
	setString("HIVE_SYNC_WORKER_INTERVAL", &cfg.HiveSync.WorkerInterval)

	if schemaPath := strings.TrimSpace(os.Getenv("NOTES_SCHEMA_PATH")); schemaPath != "" {
		cfg.NotesSchemaPath = schemaPath
	}
	if schemaDir := strings.TrimSpace(os.Getenv("SCHEMA_PATH")); schemaDir != "" {
		cfg.NotesSchemaPath = filepath.Join(schemaDir, "schema.sql")
		cfg.CommandsSchemaPath = filepath.Join(schemaDir, "command-schema.sql")
	}
}

func normalize(cfg *Config) {
	cfg.ConfigPath = ExpandPath(cfg.ConfigPath)
	cfg.NotesDir = ExpandPath(cfg.NotesDir)
	cfg.DBPath = ExpandPath(cfg.DBPath)
	cfg.CommandsDBPath = ExpandPath(cfg.CommandsDBPath)
	cfg.InboxPath = ExpandPath(cfg.InboxPath)
	cfg.DashboardPath = ExpandPath(cfg.DashboardPath)
	cfg.NotesSchemaPath = ExpandPath(cfg.NotesSchemaPath)
	cfg.CommandsSchemaPath = ExpandPath(cfg.CommandsSchemaPath)
	cfg.HiveSync.ConflictsDir = ExpandPath(cfg.HiveSync.ConflictsDir)
	if cfg.InboxPath == "" && cfg.NotesDir != "" {
		cfg.InboxPath = filepath.Join(cfg.NotesDir, "inbox.md")
	}
	if cfg.DashboardPath == "" && cfg.NotesDir != "" {
		cfg.DashboardPath = filepath.Join(cfg.NotesDir, "dashboard.md")
	}
}

func parseHiveSyncValue(key, value string, cfg *Config) error {
	switch key {
	case "enabled":
		return parseBoolInto(value, &cfg.HiveSync.Enabled)
	case "endpoint":
		return parseStringInto(value, &cfg.HiveSync.Endpoint)
	case "device_id":
		return parseStringInto(value, &cfg.HiveSync.DeviceID)
	case "device_name":
		return parseStringInto(value, &cfg.HiveSync.DeviceName)
	case "platform":
		return parseStringInto(value, &cfg.HiveSync.Platform)
	case "app_version":
		return parseStringInto(value, &cfg.HiveSync.AppVersion)
	case "token_command":
		return parseStringInto(value, &cfg.HiveSync.TokenCommand)
	case "token_from_keychain":
		return parseBoolInto(value, &cfg.HiveSync.TokenFromKeychain)
	case "token_keychain_service":
		return parseStringInto(value, &cfg.HiveSync.TokenKeychainService)
	case "token_keychain_account":
		return parseStringInto(value, &cfg.HiveSync.TokenKeychainAccount)
	case "conflicts_dir":
		return parseStringInto(value, &cfg.HiveSync.ConflictsDir)
	case "outbox_limit":
		return parseIntInto(value, &cfg.HiveSync.OutboxLimit)
	case "pull_limit":
		return parseIntInto(value, &cfg.HiveSync.PullLimit)
	case "retry_attempts":
		return parseIntInto(value, &cfg.HiveSync.RetryAttempts)
	case "retry_base_delay":
		return parseStringInto(value, &cfg.HiveSync.RetryBaseDelay)
	case "retry_max_delay":
		return parseStringInto(value, &cfg.HiveSync.RetryMaxDelay)
	case "worker_iterations":
		return parseIntInto(value, &cfg.HiveSync.WorkerIterations)
	case "until_empty":
		return parseBoolInto(value, &cfg.HiveSync.UntilEmpty)
	case "until_empty_max_iterations":
		return parseIntInto(value, &cfg.HiveSync.UntilEmptyMaxIterations)
	case "worker_interval":
		return parseStringInto(value, &cfg.HiveSync.WorkerInterval)
	}
	return nil
}

func parseHivePWAValue(key, value string, cfg *Config) error {
	switch key {
	case "enabled":
		return parseBoolInto(value, &cfg.HivePWA.Enabled)
	case "url":
		return parseStringInto(value, &cfg.HivePWA.URL)
	case "api_url":
		return parseStringInto(value, &cfg.HivePWA.APIURL)
	}
	return nil
}

func parseStringInto(value string, dest *string) error {
	parsed, err := parseStringValue(value)
	if err != nil {
		return err
	}
	*dest = parsed
	return nil
}

func parseBoolInto(value string, dest *bool) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	*dest = parsed
	return nil
}

func parseIntInto(value string, dest *int) error {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	*dest = parsed
	return nil
}

func loadDotEnvIfPresent() {
	paths := []string{".env"}
	if home := homeDir(); home != "" {
		paths = append(paths, filepath.Join(home, "Projects", "mind-weaver", ".env"))
	}
	for _, path := range paths {
		if fileExists(path) {
			_ = godotenv.Load(path)
		}
	}
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	return "."
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func pathExistsCheck(name, path string, wantDir bool) Check {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Check{Name: name, Status: CheckFail, Message: fmt.Sprintf("missing: %s", path)}
		}
		return Check{Name: name, Status: CheckFail, Message: err.Error()}
	}
	if wantDir && !info.IsDir() {
		return Check{Name: name, Status: CheckFail, Message: fmt.Sprintf("not a directory: %s", path)}
	}
	if !wantDir && info.IsDir() {
		return Check{Name: name, Status: CheckFail, Message: fmt.Sprintf("is a directory: %s", path)}
	}
	return Check{Name: name, Status: CheckOK, Message: path}
}

func parentCheck(name, path string) Check {
	if strings.TrimSpace(path) == "" {
		return Check{Name: name, Status: CheckFail, Message: "not configured"}
	}
	if fileExists(path) {
		return Check{Name: name, Status: CheckOK, Message: path}
	}
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return Check{Name: name, Status: CheckWarn, Message: fmt.Sprintf("parent missing: %s", parent)}
	}
	if !info.IsDir() {
		return Check{Name: name, Status: CheckFail, Message: fmt.Sprintf("parent is not a directory: %s", parent)}
	}
	return Check{Name: name, Status: CheckOK, Message: fmt.Sprintf("can be created: %s", path)}
}

func schemaCheck(name, path string) Check {
	if path == db.EmbeddedSchemaPath || path == db.EmbeddedCommandSchemaPath {
		return Check{Name: name, Status: CheckOK, Message: path}
	}
	if strings.TrimSpace(path) == "" {
		return Check{Name: name, Status: CheckOK, Message: db.EmbeddedSchemaPath}
	}
	return pathExistsCheck(name, path, false)
}

func hiveSyncCheck(cfg HiveSyncConfig) Check {
	if !cfg.Enabled {
		return Check{Name: "hive sync", Status: CheckOK, Message: "disabled"}
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return Check{Name: "hive sync", Status: CheckFail, Message: "enabled but endpoint is empty"}
	}
	return Check{Name: "hive sync", Status: CheckOK, Message: cfg.Endpoint}
}

func hivePWACheck(cfg HivePWAConfig) Check {
	if !cfg.Enabled {
		return Check{Name: "hive pwa", Status: CheckOK, Message: "disabled"}
	}
	if strings.TrimSpace(cfg.URL) == "" && strings.TrimSpace(cfg.APIURL) == "" {
		return Check{Name: "hive pwa", Status: CheckWarn, Message: "enabled but url/api_url are empty"}
	}
	if strings.TrimSpace(cfg.URL) != "" {
		return Check{Name: "hive pwa", Status: CheckOK, Message: cfg.URL}
	}
	return Check{Name: "hive pwa", Status: CheckOK, Message: cfg.APIURL}
}
