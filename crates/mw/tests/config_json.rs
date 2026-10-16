use std::{
    fs,
    path::{Path, PathBuf},
    process::{Command, Output},
    sync::atomic::{AtomicUsize, Ordering},
};

use serde_json::{Value, json};

struct Sandbox {
    root: PathBuf,
}

impl Sandbox {
    fn new() -> Self {
        static NEXT_ID: AtomicUsize = AtomicUsize::new(0);
        // Resolve the existing temp parent because macOS cwd resolves /var to /private/var.
        let parent = fs::canonicalize(std::env::temp_dir()).unwrap();
        let root = parent.join(format!(
            "mw-config-json-{}-{}",
            std::process::id(),
            NEXT_ID.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root).unwrap();
        for dir in ["home", "cwd", "config"] {
            fs::create_dir(root.join(dir)).unwrap();
        }
        Self { root }
    }

    fn command(&self) -> Command {
        let mut command = Command::new(env!("CARGO_BIN_EXE_mw"));
        command
            .args(["config", "json"])
            .env_clear()
            .env("HOME", self.root.join("home"))
            .env("XDG_CONFIG_HOME", self.root.join("xdg-config"))
            .env("XDG_DATA_HOME", self.root.join("xdg-data"))
            .env("XDG_STATE_HOME", self.root.join("xdg-state"))
            .env("MW_CONFIG", self.root.join("config/config.toml"))
            .current_dir(self.root.join("cwd"));
        command
    }

    fn config(&self, text: &str) {
        fs::write(self.root.join("config/config.toml"), text).unwrap();
    }

    fn assert_paths(&self, output: Output, notes: &Path, inbox: &Path, dashboard: &Path) {
        assert!(
            output.status.success(),
            "stderr: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        assert!(output.stderr.is_empty());
        let value: Value = serde_json::from_slice(&output.stdout).unwrap();
        assert_eq!(
            value,
            json!({
                "schemaVersion": "1",
                "notesDir": notes,
                "inboxPath": inbox,
                "dashboardPath": dashboard,
            })
        );
        for key in ["notesDir", "inboxPath", "dashboardPath"] {
            assert!(Path::new(value[key].as_str().unwrap()).is_absolute());
        }
    }
}

impl Drop for Sandbox {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root).unwrap();
    }
}

#[test]
fn config_json_has_only_version_and_expanded_absolute_paths() {
    let sandbox = Sandbox::new();
    sandbox.config(
        r#"notes_dir = "~/Brain"
inbox_path = "$HOME/capture.md"
dashboard_path = "${HOME}/overview.md"
db_path = "must-not-create/notes.db"
"#,
    );
    sandbox.assert_paths(
        sandbox.command().output().unwrap(),
        &sandbox.root.join("home/Brain"),
        &sandbox.root.join("home/capture.md"),
        &sandbox.root.join("home/overview.md"),
    );
    assert!(!sandbox.root.join("cwd/must-not-create").exists());
    assert!(
        fs::read_dir(sandbox.root.join("home"))
            .unwrap()
            .next()
            .is_none()
    );
}

#[test]
fn environment_overrides_config_and_mw_inbox_overrides_inbox() {
    let sandbox = Sandbox::new();
    sandbox.config(
        "notes_dir = config-notes\ninbox_path = config-inbox.md\ndashboard_path = config-dashboard.md\n",
    );
    sandbox.assert_paths(
        sandbox
            .command()
            .env("NOTES_DIR", "~/env-notes")
            .env("INBOX_PATH", "ignored-inbox.md")
            .env("MW_INBOX_PATH", "$HOME/env-inbox.md")
            .env("DASHBOARD_PATH", "~/env-dashboard.md")
            .output()
            .unwrap(),
        &sandbox.root.join("home/env-notes"),
        &sandbox.root.join("home/env-inbox.md"),
        &sandbox.root.join("home/env-dashboard.md"),
    );
}

#[test]
fn relative_paths_are_joined_to_subprocess_cwd() {
    let sandbox = Sandbox::new();
    sandbox.config(
        "notes_dir = missing-notes\ninbox_path = capture/inbox.md\ndashboard_path = ../dashboard.md\n",
    );
    sandbox.assert_paths(
        sandbox.command().output().unwrap(),
        &sandbox.root.join("cwd/missing-notes"),
        &sandbox.root.join("cwd/capture/inbox.md"),
        &sandbox.root.join("cwd/../dashboard.md"),
    );
    assert!(
        fs::read_dir(sandbox.root.join("cwd"))
            .unwrap()
            .next()
            .is_none()
    );
}

#[test]
fn missing_config_returns_defaults_without_creating_anything() {
    let sandbox = Sandbox::new();
    sandbox.assert_paths(
        sandbox.command().output().unwrap(),
        &sandbox.root.join("home/Notes"),
        &sandbox.root.join("home/Notes/inbox.md"),
        &sandbox.root.join("home/Notes/dashboard.md"),
    );
    for dir in ["home", "cwd", "config"] {
        assert!(
            fs::read_dir(sandbox.root.join(dir))
                .unwrap()
                .next()
                .is_none()
        );
    }
    assert_eq!(fs::read_dir(&sandbox.root).unwrap().count(), 3);
}

#[test]
fn malformed_config_fails_without_json_fallback() {
    let sandbox = Sandbox::new();
    sandbox.config("notes_dir = \"unterminated\n");
    let output = sandbox.command().output().unwrap();
    assert!(!output.status.success());
    assert!(output.stdout.is_empty());
    assert!(String::from_utf8_lossy(&output.stderr).contains("parse config"));
}

#[test]
fn existing_config_suppresses_dotenv() {
    let sandbox = Sandbox::new();
    sandbox.config("notes_dir = ~/configured\n");
    fs::write(
        sandbox.root.join("cwd/.env"),
        "NOTES_DIR=dotenv-notes\nMW_INBOX_PATH=dotenv-inbox.md\nDASHBOARD_PATH=dotenv-dashboard.md\n",
    )
    .unwrap();
    sandbox.assert_paths(
        sandbox.command().output().unwrap(),
        &sandbox.root.join("home/configured"),
        &sandbox.root.join("home/Notes/inbox.md"),
        &sandbox.root.join("home/Notes/dashboard.md"),
    );
}

#[test]
fn notes_override_preserves_engine_inbox_and_dashboard_defaults() {
    let sandbox = Sandbox::new();
    sandbox.assert_paths(
        sandbox
            .command()
            .env("NOTES_DIR", "alternate-notes")
            .output()
            .unwrap(),
        &sandbox.root.join("cwd/alternate-notes"),
        &sandbox.root.join("home/Notes/inbox.md"),
        &sandbox.root.join("home/Notes/dashboard.md"),
    );
}

#[test]
fn token_command_is_neither_executed_nor_surfaced() {
    let sandbox = Sandbox::new();
    let marker = sandbox.root.join("token-command-executed");
    let token_command = format!("/usr/bin/touch '{}'; printf secret-token", marker.display());
    sandbox.config(&format!(
        "[hive_sync]\nenabled = true\ntoken_from_keychain = true\ntoken_command = {}\ntoken_keychain_account = secret-account\n",
        serde_json::to_string(&token_command).unwrap()
    ));
    sandbox.assert_paths(
        sandbox.command().output().unwrap(),
        &sandbox.root.join("home/Notes"),
        &sandbox.root.join("home/Notes/inbox.md"),
        &sandbox.root.join("home/Notes/dashboard.md"),
    );
    assert!(!marker.exists());
}
