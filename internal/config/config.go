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
`, CollapseHome(cfg.NotesDir), CollapseHome(cfg.DBPath), CollapseHome(cfg.CommandsDBPath), CollapseHome(cfg.InboxPath), CollapseHome(cfg.DashboardPath))
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
	if cfg.InboxPath == "" && cfg.NotesDir != "" {
		cfg.InboxPath = filepath.Join(cfg.NotesDir, "inbox.md")
	}
	if cfg.DashboardPath == "" && cfg.NotesDir != "" {
		cfg.DashboardPath = filepath.Join(cfg.NotesDir, "dashboard.md")
	}
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
