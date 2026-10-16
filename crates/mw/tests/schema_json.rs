use std::{
    collections::BTreeMap,
    fs,
    path::{Path, PathBuf},
    process::{Command, Output},
    sync::atomic::{AtomicUsize, Ordering},
};

use serde_json::{Value, json};

struct Sandbox {
    root: PathBuf,
}

fn snapshot(root: &Path) -> BTreeMap<PathBuf, Option<Vec<u8>>> {
    fn visit(root: &Path, path: &Path, entries: &mut BTreeMap<PathBuf, Option<Vec<u8>>>) {
        for entry in fs::read_dir(path).unwrap() {
            let path = entry.unwrap().path();
            let relative = path.strip_prefix(root).unwrap().to_path_buf();
            if path.is_dir() {
                entries.insert(relative, None);
                visit(root, &path, entries);
            } else {
                entries.insert(relative, Some(fs::read(path).unwrap()));
            }
        }
    }
    let mut entries = BTreeMap::new();
    visit(root, root, &mut entries);
    entries
}

impl Sandbox {
    fn new() -> Self {
        static NEXT_ID: AtomicUsize = AtomicUsize::new(0);
        let parent = fs::canonicalize(std::env::temp_dir()).unwrap();
        let root = parent.join(format!(
            "mw-schema-json-{}-{}",
            std::process::id(),
            NEXT_ID.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root).unwrap();
        for dir in ["home", "cwd", "config"] {
            fs::create_dir(root.join(dir)).unwrap();
        }
        // Files used as directory ancestors make notes/DB/schema paths unusable.
        fs::write(root.join("blocked"), "not a directory").unwrap();
        fs::write(
            root.join("cwd/.env"),
            "NOTES_DIR='unterminated\nMW_CONFIG=/invalid\n",
        )
        .unwrap();
        Self { root }
    }

    fn command(&self, args: &[&str]) -> Command {
        let mut command = Command::new(env!("CARGO_BIN_EXE_mw"));
        command
            .args(["schema"])
            .args(args)
            .env_clear()
            .env("HOME", self.root.join("home"))
            .env("XDG_CONFIG_HOME", self.root.join("xdg-config"))
            .env("XDG_DATA_HOME", self.root.join("xdg-data"))
            .env("XDG_STATE_HOME", self.root.join("xdg-state"))
            .env("MW_CONFIG", self.root.join("config/config.toml"))
            .env("NOTES_DIR", self.root.join("blocked/notes"))
            .env("DB_PATH", self.root.join("blocked/notes.db"))
            .env("COMMANDS_DB_PATH", self.root.join("blocked/commands.db"))
            .env("SCHEMA_PATH", self.root.join("blocked/schema.sql"))
            .env("NOTES_SCHEMA_PATH", self.root.join("blocked/notes.sql"))
            .current_dir(self.root.join("cwd"));
        command
    }

    fn json(&self, args: &[&str]) -> Value {
        let before = snapshot(&self.root);
        let output = self.command(args).output().unwrap();
        assert_eq!(
            snapshot(&self.root),
            before,
            "schema command changed sandbox"
        );
        success_json(output)
    }
}

fn success_json(output: Output) -> Value {
    assert!(
        output.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert!(output.stderr.is_empty());
    serde_json::from_slice(&output.stdout).unwrap()
}

impl Drop for Sandbox {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root).unwrap();
    }
}

fn expected_list() -> Value {
    json!({"schemaVersion": "1", "schemas": [
        {"domain": "abbreviation-index", "aliases": ["abbreviationIndex"]},
        {"domain": "dev-tool", "aliases": ["devTool"]},
        {"domain": "glossary", "aliases": []},
        {"domain": "programming-concept", "aliases": ["programmingConcept"]},
        {"domain": "programming-implementation", "aliases": ["programmingImplementation"]},
        {"domain": "programming-language", "aliases": ["programmingLanguage"]},
        {"domain": "recipe", "aliases": []},
        {"domain": "task-index", "aliases": ["taskIndex"]},
        {"domain": "vocabulary-index", "aliases": ["vocabularyIndex"]},
        {"domain": "writing-character", "aliases": ["writingCharacter"]}
    ]})
}

#[test]
fn list_is_exact_sorted_and_deterministic_with_or_without_flag() {
    let sandbox = Sandbox::new();
    assert_eq!(sandbox.json(&["list", "--json"]), expected_list());
    assert_eq!(sandbox.json(&["list"]), expected_list());
    let first = sandbox.command(&["list", "--json"]).output().unwrap();
    let second = sandbox.command(&["list", "--json"]).output().unwrap();
    assert_eq!(first.stdout, second.stdout);
}

#[test]
fn all_schemas_and_aliases_have_exact_envelopes_and_full_documents() {
    let sandbox = Sandbox::new();
    for descriptor in mw_notes::list_domain_schemas() {
        let schema =
            serde_json::to_value(mw_notes::domain_schema_document(descriptor.domain).unwrap())
                .unwrap();
        let expected = json!({"schemaVersion": "1", "domain": descriptor.domain, "schema": schema});
        for name in std::iter::once(descriptor.domain.to_string())
            .chain(descriptor.aliases.iter().map(|name| name.to_string()))
            .chain([
                descriptor.domain.replace('-', "_"),
                descriptor.domain.replace('-', " "),
            ])
        {
            assert_eq!(sandbox.json(&["show", &name, "--json"]), expected);
        }
        assert_eq!(sandbox.json(&["show", descriptor.domain]), expected);
    }
    let dev = sandbox.json(&["show", "devTool", "--json"]);
    assert_eq!(dev["schema"]["version"], json!(0.1));
    assert!(dev["schema"]["meta"]["allowed"].is_array());
    assert!(dev["schema"]["naming"].is_object());
    assert!(dev["schema"]["notes"].is_array());
    let fields = &dev["schema"]["sections"]["structure"]["Commands"]["child_rules"]["fields"];
    assert_eq!(
        fields["description"],
        json!({"type": "string", "required": true})
    );
    assert_eq!(
        fields["arg_specs"]["object_shape"]["required"],
        json!(["key", "type"])
    );
    assert!(
        fields["arg_specs"]["object_shape"]["allowed"]
            .as_array()
            .unwrap()
            .contains(&json!("pattern"))
    );
    let task = sandbox.json(&["show", "task-index", "--json"]);
    assert_eq!(task["schema"]["version"], json!(0.3));
}

#[test]
fn missing_malformed_and_token_configs_are_irrelevant_and_create_nothing() {
    let sandbox = Sandbox::new();
    let token_command = format!(
        "/usr/bin/touch '{}'; printf secret-token",
        sandbox.root.join("token-executed").display()
    );
    for config in [
        None,
        Some("notes_dir = \"unterminated\n".to_string()),
        Some(format!(
            "notes_dir = \"blocked/notes\"\ndb_path = \"must-not-create/notes.db\"\n[hive_sync]\nenabled = true\ntoken_from_keychain = true\ntoken_command = {}\ntoken_keychain_account = \"secret-account\"\n",
            serde_json::to_string(&token_command).unwrap()
        )),
    ] {
        if let Some(config) = config {
            fs::write(sandbox.root.join("config/config.toml"), config).unwrap();
        }
        assert_eq!(sandbox.json(&["list", "--json"]), expected_list());
        let value = sandbox.json(&["show", "task-index", "--json"]);
        assert_eq!(value["domain"], "task-index");
        assert!(!sandbox.root.join("token-executed").exists());
    }
    let before = snapshot(&sandbox.root);
    let output = sandbox
        .command(&["show", "glossary", "--json"])
        .env("MW_CONFIG", sandbox.root.join("blocked/config.toml"))
        .output()
        .unwrap();
    assert_eq!(success_json(output)["domain"], "glossary");
    assert_eq!(snapshot(&sandbox.root), before);
}

#[test]
fn unknown_case_filenames_and_paths_fail_without_stdout_or_side_effects() {
    let sandbox = Sandbox::new();
    for name in [
        "unknown",
        "TaskIndex",
        "taskindex",
        "task-index.yaml",
        "devTool.yml",
        "../glossary",
        "/glossary",
    ] {
        let before = snapshot(&sandbox.root);
        let output = sandbox.command(&["show", name, "--json"]).output().unwrap();
        assert!(!output.status.success(), "{name}");
        assert!(output.stdout.is_empty());
        assert!(String::from_utf8_lossy(&output.stderr).contains("unknown domain schema"));
        assert_eq!(snapshot(&sandbox.root), before);
    }
}
