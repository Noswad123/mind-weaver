use std::{
    collections::HashMap,
    env, fs,
    path::{Path, PathBuf},
};

use anyhow::{Context, Result, anyhow, bail};

use crate::{APP_DIR_NAME, EMBEDDED_COMMAND_SCHEMA_PATH, EMBEDDED_SCHEMA_PATH};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Config {
    pub config_path: String,
    pub notes_dir: String,
    pub db_path: String,
    pub commands_db_path: String,
    pub inbox_path: String,
    pub dashboard_path: String,
    pub notes_schema_path: String,
    pub commands_schema_path: String,
    pub hive_sync: HiveSyncConfig,
    pub hive_pwa: HivePwaConfig,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HiveSyncConfig {
    pub enabled: bool,
    pub endpoint: String,
    pub device_id: String,
    pub device_name: String,
    pub platform: String,
    pub app_version: String,
    pub token_command: String,
    pub token_from_keychain: bool,
    pub token_keychain_service: String,
    pub token_keychain_account: String,
    pub conflicts_dir: String,
    pub outbox_limit: i64,
    pub pull_limit: i64,
    pub retry_attempts: i64,
    pub retry_base_delay: String,
    pub retry_max_delay: String,
    pub worker_iterations: i64,
    pub until_empty: bool,
    pub until_empty_max_iterations: i64,
    pub worker_interval: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct HivePwaConfig {
    pub enabled: bool,
    pub url: String,
    pub api_url: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct InitOptions {
    pub config_path: Option<String>,
    pub notes_dir: Option<String>,
    pub db_path: Option<String>,
    pub inbox_path: Option<String>,
    pub force: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CheckStatus {
    Ok,
    Warn,
    Fail,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Check {
    pub name: String,
    pub status: CheckStatus,
    pub message: String,
}

impl Default for Config {
    fn default() -> Self {
        let notes_dir = expand_path("~/Notes");
        let data_dir = data_dir();
        Self {
            config_path: default_config_path(),
            notes_dir: notes_dir.clone(),
            db_path: join(&data_dir, "mind-weaver.db"),
            commands_db_path: join(&data_dir, "command.db"),
            inbox_path: join(&notes_dir, "inbox.md"),
            dashboard_path: join(&notes_dir, "dashboard.md"),
            notes_schema_path: EMBEDDED_SCHEMA_PATH.to_string(),
            commands_schema_path: EMBEDDED_COMMAND_SCHEMA_PATH.to_string(),
            hive_sync: HiveSyncConfig::default(),
            hive_pwa: HivePwaConfig::default(),
        }
    }
}

impl Default for HiveSyncConfig {
    fn default() -> Self {
        Self {
            enabled: false,
            endpoint: "http://127.0.0.1:8080".to_string(),
            device_id: String::new(),
            device_name: String::new(),
            platform: String::new(),
            app_version: "mw-dev".to_string(),
            token_command: String::new(),
            token_from_keychain: false,
            token_keychain_service: "mw/hive-sync".to_string(),
            token_keychain_account: String::new(),
            conflicts_dir: join(&data_dir(), "conflicts"),
            outbox_limit: 100,
            pull_limit: 100,
            retry_attempts: 3,
            retry_base_delay: "500ms".to_string(),
            retry_max_delay: "5s".to_string(),
            worker_iterations: 1,
            until_empty: false,
            until_empty_max_iterations: 100,
            worker_interval: "15s".to_string(),
        }
    }
}

pub fn load() -> Result<Config> {
    let mut cfg = Config::default();
    let mut dotenv = HashMap::new();

    let mut path = env_non_empty("MW_CONFIG").unwrap_or_else(|| cfg.config_path.clone());
    path = expand_path(&path);
    cfg.config_path = path.clone();

    if !file_exists(&path) {
        dotenv = load_dotenv_if_present();
        if let Some(env_path) = env_or_dotenv("MW_CONFIG", &dotenv) {
            path = expand_path(&env_path);
            cfg.config_path = path.clone();
        }
    }

    match fs::read(&path) {
        Ok(data) => {
            parse_config(&data, &mut cfg).with_context(|| format!("parse config {}", path))?
        }
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => {}
        Err(err) => return Err(err).with_context(|| format!("read config {}", path)),
    }

    apply_env_overrides(&mut cfg, &dotenv);
    normalize(&mut cfg);
    Ok(cfg)
}

pub fn active_config_path() -> String {
    let default_path = default_config_path();
    let mut path = env_non_empty("MW_CONFIG").unwrap_or(default_path);
    path = expand_path(&path);

    if !file_exists(&path) {
        let dotenv = load_dotenv_if_present();
        if let Some(env_path) = env_or_dotenv("MW_CONFIG", &dotenv) {
            path = expand_path(&env_path);
        }
    }

    path
}

pub fn init(opts: InitOptions) -> Result<Config> {
    let mut cfg = Config::default();

    if let Some(path) = non_empty_opt(opts.config_path) {
        cfg.config_path = expand_path(&path);
    }
    if let Some(notes_dir) = non_empty_opt(opts.notes_dir) {
        cfg.notes_dir = expand_path(&notes_dir);
        if opts
            .inbox_path
            .as_deref()
            .unwrap_or_default()
            .trim()
            .is_empty()
        {
            cfg.inbox_path = join(&cfg.notes_dir, "inbox.md");
        }
        cfg.dashboard_path = join(&cfg.notes_dir, "dashboard.md");
    }
    if let Some(db_path) = non_empty_opt(opts.db_path) {
        cfg.db_path = expand_path(&db_path);
    }
    if let Some(inbox_path) = non_empty_opt(opts.inbox_path) {
        cfg.inbox_path = expand_path(&inbox_path);
    }

    normalize(&mut cfg);

    if Path::new(&cfg.config_path).exists() && !opts.force {
        bail!(
            "config already exists at {}; use --force to overwrite",
            cfg.config_path
        );
    }

    for dir in [
        parent(&cfg.config_path),
        Some(data_dir()),
        Some(state_dir()),
        parent(&cfg.db_path),
        parent(&cfg.commands_db_path),
        Some(cfg.notes_dir.clone()),
        parent(&cfg.inbox_path),
        parent(&cfg.dashboard_path),
    ]
    .into_iter()
    .flatten()
    {
        if dir.trim().is_empty() || dir == "." {
            continue;
        }
        fs::create_dir_all(&dir).with_context(|| format!("create directory {}", dir))?;
    }

    fs::write(&cfg.config_path, format_config(&cfg))
        .with_context(|| format!("write config {}", cfg.config_path))?;

    Ok(cfg)
}

pub fn format_config(cfg: &Config) -> String {
    let mut cfg = cfg.clone();
    normalize(&mut cfg);
    format!(
        "# MindWeaver configuration\n\
notes_dir = {}\n\
db_path = {}\n\
commands_db_path = {}\n\
inbox_path = {}\n\
dashboard_path = {}\n\
\n\
[hive_sync]\n\
enabled = {}\n\
endpoint = {}\n\
device_id = {}\n\
device_name = {}\n\
app_version = {}\n\
token_command = {}\n\
token_from_keychain = {}\n\
token_keychain_service = {}\n\
token_keychain_account = {}\n\
conflicts_dir = {}\n\
outbox_limit = {}\n\
pull_limit = {}\n\
retry_attempts = {}\n\
retry_base_delay = {}\n\
retry_max_delay = {}\n\
worker_iterations = {}\n\
until_empty = {}\n\
until_empty_max_iterations = {}\n\
worker_interval = {}\n\
\n\
[hive_pwa]\n\
enabled = {}\n\
url = {}\n\
api_url = {}\n",
        quote(&collapse_home(&cfg.notes_dir)),
        quote(&collapse_home(&cfg.db_path)),
        quote(&collapse_home(&cfg.commands_db_path)),
        quote(&collapse_home(&cfg.inbox_path)),
        quote(&collapse_home(&cfg.dashboard_path)),
        cfg.hive_sync.enabled,
        quote(&cfg.hive_sync.endpoint),
        quote(&cfg.hive_sync.device_id),
        quote(&cfg.hive_sync.device_name),
        quote(&cfg.hive_sync.app_version),
        quote(&cfg.hive_sync.token_command),
        cfg.hive_sync.token_from_keychain,
        quote(&cfg.hive_sync.token_keychain_service),
        quote(&cfg.hive_sync.token_keychain_account),
        quote(&collapse_home(&cfg.hive_sync.conflicts_dir)),
        cfg.hive_sync.outbox_limit,
        cfg.hive_sync.pull_limit,
        cfg.hive_sync.retry_attempts,
        quote(&cfg.hive_sync.retry_base_delay),
        quote(&cfg.hive_sync.retry_max_delay),
        cfg.hive_sync.worker_iterations,
        cfg.hive_sync.until_empty,
        cfg.hive_sync.until_empty_max_iterations,
        quote(&cfg.hive_sync.worker_interval),
        cfg.hive_pwa.enabled,
        quote(&cfg.hive_pwa.url),
        quote(&cfg.hive_pwa.api_url),
    )
}

pub fn doctor(cfg: Config, load_err: Option<&anyhow::Error>) -> Vec<Check> {
    let mut cfg = cfg;
    normalize(&mut cfg);
    let mut checks = vec![Check {
        name: "config path".to_string(),
        status: CheckStatus::Ok,
        message: cfg.config_path.clone(),
    }];

    if let Some(err) = load_err {
        checks.push(Check {
            name: "config load".to_string(),
            status: CheckStatus::Fail,
            message: err.to_string(),
        });
    } else if file_exists(&cfg.config_path) {
        checks.push(Check {
            name: "config load".to_string(),
            status: CheckStatus::Ok,
            message: "config file loaded".to_string(),
        });
    } else {
        checks.push(Check {
            name: "config load".to_string(),
            status: CheckStatus::Warn,
            message: "config file does not exist; run `mw init`".to_string(),
        });
    }

    checks.extend([
        path_exists_check("notes directory", &cfg.notes_dir, true),
        parent_check("inbox path", &cfg.inbox_path),
        parent_check("database path", &cfg.db_path),
        schema_check("notes schema", &cfg.notes_schema_path),
        schema_check("commands schema", &cfg.commands_schema_path),
        hive_sync_check(&cfg.hive_sync),
        hive_pwa_check(&cfg.hive_pwa),
    ]);

    checks
}

pub fn config_dir() -> String {
    if let Some(v) = env_non_empty("XDG_CONFIG_HOME") {
        return expand_path(&v);
    }
    join(&home_dir(), ".config")
}

pub fn data_dir() -> String {
    if let Some(v) = env_non_empty("XDG_DATA_HOME") {
        return join(&expand_path(&v), APP_DIR_NAME);
    }
    join(&join(&home_dir(), ".local/share"), APP_DIR_NAME)
}

pub fn state_dir() -> String {
    if let Some(v) = env_non_empty("XDG_STATE_HOME") {
        return join(&expand_path(&v), APP_DIR_NAME);
    }
    join(&join(&home_dir(), ".local/state"), APP_DIR_NAME)
}

pub fn default_config_path() -> String {
    join(&join(&config_dir(), APP_DIR_NAME), "config.toml")
}

pub fn expand_path(path: &str) -> String {
    let path = expand_env_vars(path.trim());
    if path == "~" {
        return home_dir();
    }
    if let Some(rest) = path.strip_prefix("~/") {
        return join(&home_dir(), rest);
    }
    path
}

pub fn collapse_home(path: &str) -> String {
    let home = home_dir();
    if path == home {
        return "~".to_string();
    }
    let prefix = format!("{}{}", home, std::path::MAIN_SEPARATOR);
    if let Some(rest) = path.strip_prefix(&prefix) {
        return format!("~/{}", rest);
    }
    path.to_string()
}

fn parse_config(data: &[u8], cfg: &mut Config) -> Result<()> {
    let text = std::str::from_utf8(data).context("config is not valid UTF-8")?;
    let mut section = String::new();

    for (idx, raw_line) in text.lines().enumerate() {
        let line_no = idx + 1;
        let line = strip_comment(raw_line).trim().to_string();
        if line.is_empty() {
            continue;
        }
        if line.starts_with('[') && line.ends_with(']') {
            section = line
                .trim_start_matches('[')
                .trim_end_matches(']')
                .trim()
                .to_string();
            continue;
        }

        let (key, value) = line
            .split_once('=')
            .ok_or_else(|| anyhow!("line {}: expected key = value", line_no))?;
        let key = key.trim();
        let value = value.trim();

        match section.as_str() {
            "" => {
                let parsed =
                    parse_string_value(value).with_context(|| format!("line {}", line_no))?;
                match key {
                    "notes_dir" => cfg.notes_dir = parsed,
                    "db_path" => cfg.db_path = parsed,
                    "commands_db_path" => cfg.commands_db_path = parsed,
                    "inbox_path" => cfg.inbox_path = parsed,
                    "dashboard_path" => cfg.dashboard_path = parsed,
                    "notes_schema_path" => cfg.notes_schema_path = parsed,
                    "commands_schema_path" => cfg.commands_schema_path = parsed,
                    _ => {}
                }
            }
            "hive_sync" => parse_hive_sync_value(key, value, cfg)
                .with_context(|| format!("line {}", line_no))?,
            "hive_pwa" => parse_hive_pwa_value(key, value, cfg)
                .with_context(|| format!("line {}", line_no))?,
            _ => {}
        }
    }
    Ok(())
}

fn parse_hive_sync_value(key: &str, value: &str, cfg: &mut Config) -> Result<()> {
    match key {
        "enabled" => cfg.hive_sync.enabled = value.parse()?,
        "endpoint" => cfg.hive_sync.endpoint = parse_string_value(value)?,
        "device_id" => cfg.hive_sync.device_id = parse_string_value(value)?,
        "device_name" => cfg.hive_sync.device_name = parse_string_value(value)?,
        "platform" => cfg.hive_sync.platform = parse_string_value(value)?,
        "app_version" => cfg.hive_sync.app_version = parse_string_value(value)?,
        "token_command" => cfg.hive_sync.token_command = parse_string_value(value)?,
        "token_from_keychain" => cfg.hive_sync.token_from_keychain = value.parse()?,
        "token_keychain_service" => {
            cfg.hive_sync.token_keychain_service = parse_string_value(value)?
        }
        "token_keychain_account" => {
            cfg.hive_sync.token_keychain_account = parse_string_value(value)?
        }
        "conflicts_dir" => cfg.hive_sync.conflicts_dir = parse_string_value(value)?,
        "outbox_limit" => cfg.hive_sync.outbox_limit = value.parse()?,
        "pull_limit" => cfg.hive_sync.pull_limit = value.parse()?,
        "retry_attempts" => cfg.hive_sync.retry_attempts = value.parse()?,
        "retry_base_delay" => cfg.hive_sync.retry_base_delay = parse_string_value(value)?,
        "retry_max_delay" => cfg.hive_sync.retry_max_delay = parse_string_value(value)?,
        "worker_iterations" => cfg.hive_sync.worker_iterations = value.parse()?,
        "until_empty" => cfg.hive_sync.until_empty = value.parse()?,
        "until_empty_max_iterations" => cfg.hive_sync.until_empty_max_iterations = value.parse()?,
        "worker_interval" => cfg.hive_sync.worker_interval = parse_string_value(value)?,
        _ => {}
    }
    Ok(())
}

fn parse_hive_pwa_value(key: &str, value: &str, cfg: &mut Config) -> Result<()> {
    match key {
        "enabled" => cfg.hive_pwa.enabled = value.parse()?,
        "url" => cfg.hive_pwa.url = parse_string_value(value)?,
        "api_url" => cfg.hive_pwa.api_url = parse_string_value(value)?,
        _ => {}
    }
    Ok(())
}

fn parse_string_value(value: &str) -> Result<String> {
    if value.starts_with('"') {
        Ok(serde_json::from_str(value)?)
    } else {
        Ok(value.trim().to_string())
    }
}

fn strip_comment(line: &str) -> String {
    let mut in_string = false;
    let mut escaped = false;
    for (idx, ch) in line.char_indices() {
        if escaped {
            escaped = false;
            continue;
        }
        if ch == '\\' && in_string {
            escaped = true;
            continue;
        }
        if ch == '"' {
            in_string = !in_string;
            continue;
        }
        if ch == '#' && !in_string {
            return line[..idx].to_string();
        }
    }
    line.to_string()
}

fn apply_env_overrides(cfg: &mut Config, dotenv: &HashMap<String, String>) {
    set_string("NOTES_DIR", &mut cfg.notes_dir, dotenv);
    set_string("NOTES_DB_PATH", &mut cfg.db_path, dotenv);
    set_string("COMMANDS_DB_PATH", &mut cfg.commands_db_path, dotenv);
    set_string("INBOX_PATH", &mut cfg.inbox_path, dotenv);
    set_string("MW_INBOX_PATH", &mut cfg.inbox_path, dotenv);
    set_string("DASHBOARD_PATH", &mut cfg.dashboard_path, dotenv);
    set_string("HIVE_SYNC_API_URL", &mut cfg.hive_sync.endpoint, dotenv);
    set_string("HIVE_SYNC_DEVICE_ID", &mut cfg.hive_sync.device_id, dotenv);
    set_string(
        "HIVE_SYNC_DEVICE_NAME",
        &mut cfg.hive_sync.device_name,
        dotenv,
    );
    set_string("HIVE_SYNC_PLATFORM", &mut cfg.hive_sync.platform, dotenv);
    set_string(
        "HIVE_SYNC_APP_VERSION",
        &mut cfg.hive_sync.app_version,
        dotenv,
    );
    set_string(
        "HIVE_SYNC_TOKEN_COMMAND",
        &mut cfg.hive_sync.token_command,
        dotenv,
    );
    set_string(
        "HIVE_SYNC_TOKEN_KEYCHAIN_SERVICE",
        &mut cfg.hive_sync.token_keychain_service,
        dotenv,
    );
    set_string(
        "HIVE_SYNC_TOKEN_KEYCHAIN_ACCOUNT",
        &mut cfg.hive_sync.token_keychain_account,
        dotenv,
    );
    set_string(
        "HIVE_SYNC_CONFLICTS_DIR",
        &mut cfg.hive_sync.conflicts_dir,
        dotenv,
    );
    set_string("HIVE_PWA_URL", &mut cfg.hive_pwa.url, dotenv);
    set_string("VITE_HIVE_SYNC_API_URL", &mut cfg.hive_pwa.api_url, dotenv);

    set_bool("HIVE_SYNC_ENABLED", &mut cfg.hive_sync.enabled, dotenv);
    set_bool(
        "HIVE_SYNC_TOKEN_FROM_KEYCHAIN",
        &mut cfg.hive_sync.token_from_keychain,
        dotenv,
    );
    set_bool(
        "HIVE_SYNC_UNTIL_EMPTY",
        &mut cfg.hive_sync.until_empty,
        dotenv,
    );
    set_bool("HIVE_PWA_ENABLED", &mut cfg.hive_pwa.enabled, dotenv);

    set_i64(
        "HIVE_SYNC_OUTBOX_LIMIT",
        &mut cfg.hive_sync.outbox_limit,
        dotenv,
    );
    set_i64(
        "HIVE_SYNC_PULL_LIMIT",
        &mut cfg.hive_sync.pull_limit,
        dotenv,
    );
    set_i64(
        "HIVE_SYNC_RETRY_ATTEMPTS",
        &mut cfg.hive_sync.retry_attempts,
        dotenv,
    );
    set_i64(
        "HIVE_SYNC_WORKER_ITERATIONS",
        &mut cfg.hive_sync.worker_iterations,
        dotenv,
    );
    set_i64(
        "HIVE_SYNC_UNTIL_EMPTY_MAX_ITERATIONS",
        &mut cfg.hive_sync.until_empty_max_iterations,
        dotenv,
    );

    set_string(
        "HIVE_SYNC_RETRY_BASE_DELAY",
        &mut cfg.hive_sync.retry_base_delay,
        dotenv,
    );
    set_string(
        "HIVE_SYNC_RETRY_MAX_DELAY",
        &mut cfg.hive_sync.retry_max_delay,
        dotenv,
    );
    set_string(
        "HIVE_SYNC_WORKER_INTERVAL",
        &mut cfg.hive_sync.worker_interval,
        dotenv,
    );

    set_string("NOTES_SCHEMA_PATH", &mut cfg.notes_schema_path, dotenv);
    if let Some(schema_dir) = env_or_dotenv("SCHEMA_PATH", dotenv) {
        cfg.notes_schema_path = join(&schema_dir, "schema.sql");
        cfg.commands_schema_path = join(&schema_dir, "command-schema.sql");
    }
}

fn normalize(cfg: &mut Config) {
    cfg.config_path = expand_path(&cfg.config_path);
    cfg.notes_dir = expand_path(&cfg.notes_dir);
    cfg.db_path = expand_path(&cfg.db_path);
    cfg.commands_db_path = expand_path(&cfg.commands_db_path);
    cfg.inbox_path = expand_path(&cfg.inbox_path);
    cfg.dashboard_path = expand_path(&cfg.dashboard_path);
    cfg.notes_schema_path = expand_path(&cfg.notes_schema_path);
    cfg.commands_schema_path = expand_path(&cfg.commands_schema_path);
    cfg.hive_sync.conflicts_dir = expand_path(&cfg.hive_sync.conflicts_dir);
    if cfg.inbox_path.is_empty() && !cfg.notes_dir.is_empty() {
        cfg.inbox_path = join(&cfg.notes_dir, "inbox.md");
    }
    if cfg.dashboard_path.is_empty() && !cfg.notes_dir.is_empty() {
        cfg.dashboard_path = join(&cfg.notes_dir, "dashboard.md");
    }
}

fn set_string(env_name: &str, dest: &mut String, dotenv: &HashMap<String, String>) {
    if let Some(v) = env_or_dotenv(env_name, dotenv) {
        *dest = v;
    }
}

fn set_bool(env_name: &str, dest: &mut bool, dotenv: &HashMap<String, String>) {
    if let Some(v) = env_or_dotenv(env_name, dotenv).and_then(|v| v.parse().ok()) {
        *dest = v;
    }
}

fn set_i64(env_name: &str, dest: &mut i64, dotenv: &HashMap<String, String>) {
    if let Some(v) = env_or_dotenv(env_name, dotenv).and_then(|v| v.parse().ok()) {
        *dest = v;
    }
}

fn load_dotenv_if_present() -> HashMap<String, String> {
    let mut out = HashMap::new();
    for path in [
        ".env".to_string(),
        join(&join(&home_dir(), "Projects/mind-weaver"), ".env"),
    ] {
        let Ok(text) = fs::read_to_string(path) else {
            continue;
        };
        for line in text.lines() {
            let line = strip_comment(line).trim().to_string();
            if line.is_empty() || line.starts_with("export ") && !line.contains('=') {
                continue;
            }
            let line = line.strip_prefix("export ").unwrap_or(&line);
            if let Some((key, value)) = line.split_once('=') {
                let key = key.trim().to_string();
                if env_non_empty(&key).is_none() && !out.contains_key(&key) {
                    let parsed = parse_string_value(value.trim())
                        .unwrap_or_else(|_| value.trim().to_string());
                    out.insert(key, parsed);
                }
            }
        }
    }
    out
}

fn env_or_dotenv(name: &str, dotenv: &HashMap<String, String>) -> Option<String> {
    env_non_empty(name).or_else(|| dotenv.get(name).filter(|v| !v.trim().is_empty()).cloned())
}

fn env_non_empty(name: &str) -> Option<String> {
    env::var(name).ok().filter(|v| !v.trim().is_empty())
}

fn path_exists_check(name: &str, path: &str, want_dir: bool) -> Check {
    match fs::metadata(path) {
        Ok(info) if want_dir && !info.is_dir() => {
            Check::fail(name, format!("not a directory: {path}"))
        }
        Ok(info) if !want_dir && info.is_dir() => {
            Check::fail(name, format!("is a directory: {path}"))
        }
        Ok(_) => Check::ok(name, path),
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => {
            Check::fail(name, format!("missing: {path}"))
        }
        Err(err) => Check::fail(name, err.to_string()),
    }
}

fn parent_check(name: &str, path: &str) -> Check {
    if path.trim().is_empty() {
        return Check::fail(name, "not configured");
    }
    if file_exists(path) {
        return Check::ok(name, path);
    }
    let parent = parent(path).unwrap_or_else(|| ".".to_string());
    match fs::metadata(&parent) {
        Ok(info) if !info.is_dir() => {
            Check::fail(name, format!("parent is not a directory: {parent}"))
        }
        Ok(_) => Check::ok(name, format!("can be created: {path}")),
        Err(_) => Check::warn(name, format!("parent missing: {parent}")),
    }
}

fn schema_check(name: &str, path: &str) -> Check {
    if path == EMBEDDED_SCHEMA_PATH || path == EMBEDDED_COMMAND_SCHEMA_PATH {
        return Check::ok(name, path);
    }
    if path.trim().is_empty() {
        return Check::ok(name, EMBEDDED_SCHEMA_PATH);
    }
    path_exists_check(name, path, false)
}

fn hive_sync_check(cfg: &HiveSyncConfig) -> Check {
    if !cfg.enabled {
        return Check::ok("hive sync", "disabled");
    }
    if cfg.endpoint.trim().is_empty() {
        return Check::fail("hive sync", "enabled but endpoint is empty");
    }
    Check::ok("hive sync", &cfg.endpoint)
}

fn hive_pwa_check(cfg: &HivePwaConfig) -> Check {
    if !cfg.enabled {
        return Check::ok("hive pwa", "disabled");
    }
    if cfg.url.trim().is_empty() && cfg.api_url.trim().is_empty() {
        return Check::warn("hive pwa", "enabled but url/api_url are empty");
    }
    if !cfg.url.trim().is_empty() {
        return Check::ok("hive pwa", &cfg.url);
    }
    Check::ok("hive pwa", &cfg.api_url)
}

impl Check {
    fn ok(name: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            status: CheckStatus::Ok,
            message: message.into(),
        }
    }

    fn warn(name: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            status: CheckStatus::Warn,
            message: message.into(),
        }
    }

    fn fail(name: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            status: CheckStatus::Fail,
            message: message.into(),
        }
    }
}

fn file_exists(path: &str) -> bool {
    !path.trim().is_empty() && fs::metadata(path).is_ok()
}

fn home_dir() -> String {
    env::var("HOME")
        .ok()
        .filter(|v| !v.trim().is_empty())
        .unwrap_or_else(|| ".".to_string())
}

fn parent(path: &str) -> Option<String> {
    Path::new(path)
        .parent()
        .map(|p| pathbuf_to_string(p.to_path_buf()))
}

fn join(base: &str, child: &str) -> String {
    pathbuf_to_string(Path::new(base).join(child))
}

fn pathbuf_to_string(path: PathBuf) -> String {
    path.to_string_lossy().into_owned()
}

fn non_empty_opt(value: Option<String>) -> Option<String> {
    value.and_then(|v| {
        let trimmed = v.trim().to_string();
        if trimmed.is_empty() {
            None
        } else {
            Some(trimmed)
        }
    })
}

fn quote(value: &str) -> String {
    serde_json::to_string(value).expect("string serialization should not fail")
}

fn expand_env_vars(input: &str) -> String {
    let mut out = String::new();
    let mut chars = input.chars().peekable();
    while let Some(ch) = chars.next() {
        if ch != '$' {
            out.push(ch);
            continue;
        }

        if chars.peek() == Some(&'{') {
            chars.next();
            let mut name = String::new();
            for next in chars.by_ref() {
                if next == '}' {
                    break;
                }
                name.push(next);
            }
            out.push_str(&env::var(name).unwrap_or_default());
            continue;
        }

        let mut name = String::new();
        while let Some(next) = chars.peek().copied() {
            if next == '_' || next.is_ascii_alphanumeric() {
                name.push(next);
                chars.next();
            } else {
                break;
            }
        }
        if name.is_empty() {
            out.push('$');
        } else {
            out.push_str(&env::var(name).unwrap_or_default());
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn strips_comments_outside_strings() {
        assert_eq!(
            strip_comment("notes_dir = \"#not-comment\" # comment").trim(),
            "notes_dir = \"#not-comment\""
        );
    }

    #[test]
    fn parses_basic_config_values() {
        let mut cfg = Config::default();
        parse_config(
            br#"
notes_dir = "~/Brain"
db_path = "/tmp/mw.db"

[hive_sync]
enabled = true
outbox_limit = 42

[hive_pwa]
enabled = true
url = "http://example.test"
"#,
            &mut cfg,
        )
        .unwrap();
        normalize(&mut cfg);
        assert!(cfg.notes_dir.ends_with("Brain"));
        assert_eq!(cfg.db_path, "/tmp/mw.db");
        assert!(cfg.hive_sync.enabled);
        assert_eq!(cfg.hive_sync.outbox_limit, 42);
        assert_eq!(cfg.hive_pwa.url, "http://example.test");
    }

    #[test]
    fn formats_expected_top_level_keys() {
        let cfg = Config::default();
        let formatted = format_config(&cfg);
        assert!(formatted.contains("notes_dir = "));
        assert!(formatted.contains("[hive_sync]"));
        assert!(formatted.contains("[hive_pwa]"));
    }
}
