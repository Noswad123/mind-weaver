use serde_json::{Value, json};
use std::{
    collections::BTreeMap,
    fs,
    path::{Path, PathBuf},
    process::{Command, Output},
    sync::atomic::{AtomicUsize, Ordering},
};

struct Sandbox {
    root: PathBuf,
}
impl Sandbox {
    fn new() -> Self {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let root = fs::canonicalize(std::env::temp_dir())
            .unwrap()
            .join(format!(
                "mw-todos-safety-{}-{}",
                std::process::id(),
                NEXT.fetch_add(1, Ordering::Relaxed)
            ));
        fs::create_dir(&root).unwrap();
        for dir in ["notes", "home", "config"] {
            fs::create_dir(root.join(dir)).unwrap();
        }
        fs::write(
            root.join("config/config.toml"),
            format!(
                "notes_dir = {}\ndashboard_path = {}\ndb_path = {}\n",
                root.join("notes").display(),
                root.join("notes/dashboard.md").display(),
                root.join("index.db").display()
            ),
        )
        .unwrap();
        fs::write(root.join("notes/tasks.md"), "---\nid: source\ndomains: [task]\ntask_active: true\n---\n## Todo\n### Do Now\n- [ ] One\n- [ ] Two\n").unwrap();
        Self { root }
    }
    fn run(&self, args: &[&str]) -> Output {
        Command::new(env!("CARGO_BIN_EXE_mw"))
            .args(args)
            .env_clear()
            .env("HOME", self.root.join("home"))
            .env("MW_CONFIG", self.root.join("config/config.toml"))
            .env("XDG_CONFIG_HOME", self.root.join("config"))
            .env("XDG_DATA_HOME", self.root.join("data"))
            .env("XDG_STATE_HOME", self.root.join("state"))
            .current_dir(&self.root)
            .output()
            .unwrap()
    }
    fn ok(&self, args: &[&str]) -> Output {
        let output = self.run(args);
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        output
    }
    fn query(&self) -> Vec<Value> {
        serde_json::from_slice::<Vec<Value>>(&self.ok(&["query", "todos"]).stdout).unwrap()
    }
    fn note(&self) -> Vec<u8> {
        fs::read(self.root.join("notes/tasks.md")).unwrap()
    }
    fn dashboard(&self) -> Vec<u8> {
        fs::read(self.root.join("notes/dashboard.md")).unwrap()
    }
}
impl Drop for Sandbox {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root).unwrap();
    }
}

fn fixture_tree(root: &Path) -> BTreeMap<PathBuf, Option<Vec<u8>>> {
    let mut entries = BTreeMap::new();
    for entry in fs::read_dir(root).unwrap() {
        let path = entry.unwrap().path();
        if path.is_dir() {
            entries.insert(path.clone(), None);
            entries.extend(fixture_tree(&path));
        } else {
            entries.insert(path.clone(), Some(fs::read(path).unwrap()));
        }
    }
    entries
}

#[test]
fn saved_snapshot_has_exact_typed_envelope_without_database_or_filesystem_writes() {
    for database in ["missing/index.db", "unusable-db", "invalid.db"] {
        let s = Sandbox::new();
        if database == "unusable-db" {
            fs::create_dir(s.root.join(database)).unwrap();
        }
        if database == "invalid.db" {
            fs::write(s.root.join(database), "not a SQLite database").unwrap();
        }
        fs::write(s.root.join("config/config.toml"), format!("notes_dir = notes\ndashboard_path = notes/dashboard.md\ndb_path = {database}\nnotes_schema_path = missing/schema.sql\n")).unwrap();
        fs::write(
            s.root.join("notes/tasks.md"),
            String::from_utf8(s.note()).unwrap().replace(
                "- [ ] One\n",
                "- [ ] One\n  <!-- mw-task:v1:0123456789abcdef0123456789abcdef -->\n",
            ),
        )
        .unwrap();
        fs::write(
            s.root.join("notes/dashboard.md"),
            "legacy dashboard and personal prose\r\n- [x] Do not write this back\r\n",
        )
        .unwrap();
        let expected_items = mw_notes::list_active_task_index_todos(s.root.join("notes"))
            .unwrap()
            .0;
        let before = fixture_tree(&s.root);
        for args in [
            vec!["todos", "snapshot", "--json"],
            vec!["todos", "snapshot"],
        ] {
            let output = s.ok(&args);
            assert!(output.stderr.is_empty());
            let envelope: Value = serde_json::from_slice(&output.stdout).unwrap();
            assert_eq!(
                envelope,
                json!({
                    "schemaVersion": "1", "source": "saved-markdown",
                    "notesDir": s.root.join("notes"),
                    "dashboardPath": s.root.join("notes/dashboard.md"),
                    "items": expected_items,
                })
            );
            assert_eq!(envelope.as_object().unwrap().len(), 5);
            let items: Vec<mw_notes::TaskIndexTodo> =
                serde_json::from_value(envelope["items"].clone()).unwrap();
            assert_eq!(items.len(), 2);
            assert_eq!(
                items[0].stable_id.as_deref(),
                Some("0123456789abcdef0123456789abcdef")
            );
            assert_eq!(items[0].identity_kind, "stable");
            assert_eq!(items[0].id, "task:0123456789abcdef0123456789abcdef");
            assert!(items[1].stable_id.is_none());
            assert_eq!(items[1].identity_kind, "positional");
            assert_eq!(items[1].id, "source:10:1");
            for item in items {
                let hash = item.source_revision.strip_prefix("sha256:").unwrap();
                assert_eq!(hash.len(), 64);
                assert!(
                    hash.bytes()
                        .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
                );
            }
            assert_eq!(fixture_tree(&s.root), before);
            assert!(!s.root.join("notes/.mw").exists());
            assert!(!s.root.join("missing").exists());
        }
    }
}

#[test]
fn saved_snapshot_rejects_identity_errors_without_empty_success_or_side_effects() {
    for failure in ["malformed", "duplicate-task", "duplicate-source"] {
        let s = Sandbox::new();
        let tasks = if failure == "malformed" {
            String::from_utf8(s.note())
                .unwrap()
                .replace("- [ ] One\n", "- [ ] One\n  <!-- mw-task:v1:BAD -->\n")
        } else if failure == "duplicate-task" {
            String::from_utf8(s.note())
                .unwrap()
                .replace(
                    "- [ ] One\n",
                    "- [ ] One\n  <!-- mw-task:v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n",
                )
                .replace(
                    "- [ ] Two\n",
                    "- [ ] Two\n  <!-- mw-task:v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n",
                )
        } else {
            fs::write(s.root.join("notes/duplicate.md"), s.note()).unwrap();
            String::from_utf8(s.note()).unwrap()
        };
        fs::write(s.root.join("notes/tasks.md"), tasks).unwrap();
        let before = fixture_tree(&s.root);
        let output = s.run(&["todos", "snapshot", "--json"]);
        assert!(!output.status.success(), "{failure}");
        assert!(output.stdout.is_empty());
        assert!(!output.stderr.is_empty());
        assert_eq!(fixture_tree(&s.root), before);
    }
}

#[test]
fn saved_snapshot_missing_notes_does_not_create_root_or_database() {
    let s = Sandbox::new();
    fs::write(s.root.join("config/config.toml"), "notes_dir = missing-notes\ndashboard_path = missing-dashboard/dashboard.md\ndb_path = missing-db/index.db\n").unwrap();
    let before = fixture_tree(&s.root);
    let output = s.run(&["todos", "snapshot", "--json"]);
    assert!(!output.status.success());
    assert!(output.stdout.is_empty());
    assert_eq!(fixture_tree(&s.root), before);
}

#[test]
fn snapshot_task_authority_cannot_be_overridden_by_scalar_operational_examples() {
    for newline in ["\n", "\r\n"] {
        for scalar in ["|", ">"] {
            for active in [false, true] {
                let s = Sandbox::new();
                let prefix = format!("---\nid: real-source\ndomains: [task]\ntask_active: {active}\ntask_area: Code\ntask_default_priority: p2\ntask_default_energy: small\ntitle: Real title\ntask_scope: project\ndescription: {scalar}\n  id: fake-source\n  task_active: {}\n  domains: [task]\n  task_area: Music\n  task_default_priority: p5\n  task_default_energy: xl\n  title: Wrong title\n  task_scope: fake\n  ## Todo\n  ### Do Now\n  - [ ] Example\n    <!-- mw-task:v1:BAD -->\n---\n", !active).replace('\n', newline);
                fs::write(
                    s.root.join("notes/tasks.md"),
                    prefix.clone() + &"## Todo\n### Do Now\n- [ ] Real\n".replace('\n', newline),
                )
                .unwrap();
                let before = fixture_tree(&s.root);
                let snapshot: Value =
                    serde_json::from_slice(&s.ok(&["todos", "snapshot", "--json"]).stdout).unwrap();
                assert_eq!(fixture_tree(&s.root), before);
                if !active {
                    assert!(snapshot["items"].as_array().unwrap().is_empty());
                    continue;
                }
                assert_eq!(snapshot["items"].as_array().unwrap().len(), 1);
                let real = &snapshot["items"][0];
                assert_eq!(real["sourceId"], "real-source");
                assert_eq!(real["area"], "Code");
                assert_eq!(real["noteTitle"], "Real title");
                assert_eq!(real["taskScope"], "project");
                assert_eq!(real["metadata"]["priority"], "p2");
                assert_eq!(real["metadata"]["energy"], "small");
                let marked: Value = serde_json::from_slice(
                    &s.ok(&[
                        "todos",
                        "identify",
                        "--id",
                        real["id"].as_str().unwrap(),
                        "--expected-revision",
                        real["sourceRevision"].as_str().unwrap(),
                    ])
                    .stdout,
                )
                .unwrap();
                s.ok(&[
                    "todos",
                    "toggle",
                    "--id",
                    marked["id"].as_str().unwrap(),
                    "--expected-revision",
                    marked["sourceRevision"].as_str().unwrap(),
                ]);
                assert!(s.note().starts_with(prefix.as_bytes()));
                let snapshot: Value =
                    serde_json::from_slice(&s.ok(&["todos", "snapshot", "--json"]).stdout).unwrap();
                assert_eq!(snapshot["items"][0]["sourceId"], "real-source");
                assert_eq!(snapshot["items"][0]["area"], "Code");
                assert_eq!(snapshot["items"][0]["done"], true);
                assert!(!s.root.join("index.db").exists());
            }
        }
    }
}

#[test]
fn snapshot_rejects_plus_and_ordered_nested_checkbox_attachments() {
    for bullet in ["+", "1.", "1)"] {
        for marked in [false, true] {
            let s = Sandbox::new();
            let tasks = String::from_utf8(s.note()).unwrap().replace(
                "- [ ] One\n",
                &format!(
                    "- [ ] One\n  {bullet} [ ] Child\n{}",
                    if marked {
                        "    <!-- mw-task:v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n"
                    } else {
                        ""
                    }
                ),
            );
            fs::write(s.root.join("notes/tasks.md"), tasks).unwrap();
            let before = fixture_tree(&s.root);
            let output = s.run(&["todos", "snapshot", "--json"]);
            assert!(!output.status.success());
            assert!(output.stdout.is_empty());
            let error = String::from_utf8_lossy(&output.stderr);
            assert!(error.contains("tasks.md") && error.contains("nested checkbox"));
            assert_eq!(fixture_tree(&s.root), before);
        }
    }
}

#[test]
fn dashboard_layout_edits_block_all_cli_mutations_even_with_other_pending_checkboxes() {
    for edit in ["delete", "move", "reorder", "legacy"] {
        let s = Sandbox::new();
        fs::write(s.root.join("notes/tasks.md"), String::from_utf8(s.note()).unwrap().replace("- [ ] One\n- [ ] Two\n", "- [ ] One\n  - area: Code\n- [ ] Two\n  - area: Code\n- [ ] Three\n  - area: Reading\n")).unwrap();
        s.ok(&["todos", "sync", "--require-task-safety-v1"]);
        let snapshot: Value =
            serde_json::from_slice(&s.ok(&["todos", "snapshot", "--json"]).stdout).unwrap();
        let task = &snapshot["items"][0];
        let id = task["id"].as_str().unwrap();
        let rev = task["sourceRevision"].as_str().unwrap();
        let mut lines: Vec<_> = String::from_utf8(s.dashboard())
            .unwrap()
            .replace("## Reading\n- [ ]", "## Reading\n- [x]")
            .lines()
            .map(str::to_string)
            .collect();
        let rows: Vec<_> = lines
            .iter()
            .enumerate()
            .filter(|(_, l)| l.starts_with("- ["))
            .map(|(i, _)| i)
            .collect();
        match edit {
            "delete" => {
                lines.remove(rows[0]);
            }
            "reorder" => lines.swap(rows[0], rows[1]),
            "move" => {
                let row = lines.remove(rows[0]);
                let area = lines.iter().position(|l| l == "## Reading").unwrap();
                lines.insert(area + 1, row);
            }
            "legacy" => lines.retain(|l| !l.starts_with("<!-- mw-dashboard-layout:")),
            _ => unreachable!(),
        }
        fs::write(s.root.join("notes/dashboard.md"), lines.join("\n")).unwrap();
        let before = (s.note(), s.dashboard());
        for args in [
            vec!["todos", "sync", "--require-task-safety-v1"],
            vec!["todos", "refresh"],
            vec!["todos", "toggle", "--id", id, "--expected-revision", rev],
            vec![
                "todos",
                "update",
                "--id",
                id,
                "--expected-revision",
                rev,
                "--priority",
                "p1",
            ],
            vec!["todos", "identify", "--id", id, "--expected-revision", rev],
        ] {
            let output = s.run(&args);
            assert!(!output.status.success(), "{edit} {args:?}");
            assert!(String::from_utf8_lossy(&output.stderr).contains("layout integrity"));
            assert_eq!((s.note(), s.dashboard()), before);
        }
        s.ok(&["todos", "refresh", "--discard-dashboard-edits"]);
        assert_eq!(s.note(), before.0);
        assert!(
            String::from_utf8(s.dashboard())
                .unwrap()
                .contains("<!-- mw-dashboard-layout:v1:sha256:")
        );
    }
}

#[test]
fn saved_snapshot_and_checked_mutations_exclude_frontmatter_examples() {
    for newline in ["\n", "\r\n"] {
        for scalar in ["|", ">"] {
            for marker in [
                "    <!-- mw-task:v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n",
                "    <!-- mw-task:v1:BAD -->\n",
            ] {
                let s = Sandbox::new();
                let prefix = format!("---\nid: source\ndomains: [task]\ntask_active: true\ndescription: {scalar}\n  Before\n  ## Todo\n  ### Do Now\n  - [ ] Example\n{marker}  ---\n  After\n---\n").replace('\n', newline);
                let body = "## Todo\n### Do Now\n- [ ] Real\n".replace('\n', newline);
                fs::write(s.root.join("notes/tasks.md"), prefix.clone() + &body).unwrap();
                let before = fixture_tree(&s.root);
                let snapshot: Value =
                    serde_json::from_slice(&s.ok(&["todos", "snapshot", "--json"]).stdout).unwrap();
                assert_eq!(fixture_tree(&s.root), before);
                let items = snapshot["items"].as_array().unwrap();
                assert_eq!(items.len(), 1);
                let real = &items[0];
                assert_eq!(real["text"], "Real");
                assert_eq!(real["line"], prefix.lines().count() + 3);
                assert_eq!(real["order"], 0);
                assert_eq!(real["identityKind"], "positional");
                assert!(real["stableId"].is_null());
                let marked: Value = serde_json::from_slice(
                    &s.ok(&[
                        "todos",
                        "identify",
                        "--id",
                        real["id"].as_str().unwrap(),
                        "--expected-revision",
                        real["sourceRevision"].as_str().unwrap(),
                    ])
                    .stdout,
                )
                .unwrap();
                assert!(s.note().starts_with(prefix.as_bytes()));
                let after_identify = String::from_utf8(s.note()).unwrap();
                s.ok(&[
                    "todos",
                    "toggle",
                    "--id",
                    marked["id"].as_str().unwrap(),
                    "--expected-revision",
                    marked["sourceRevision"].as_str().unwrap(),
                ]);
                assert_eq!(
                    String::from_utf8(s.note()).unwrap(),
                    after_identify.replacen("- [ ] Real", "- [x] Real", 1)
                );
                assert!(s.note().starts_with(prefix.as_bytes()));
                assert!(!s.root.join("index.db").exists());
            }
        }
    }
}

#[test]
fn mutation_flags_are_required_before_config_or_notes_access() {
    let s = Sandbox::new();
    let before = s.note();
    for args in [
        vec!["todos", "toggle", "--id", "source:8:0"],
        vec!["todos", "update", "--id", "source:8:0", "--priority", "p1"],
        vec!["todos", "identify", "--id", "source:8:0"],
    ] {
        let output = s.run(&args);
        assert!(!output.status.success());
        assert!(String::from_utf8_lossy(&output.stderr).contains("--expected-revision"));
        assert_eq!(s.note(), before);
    }
    let help = s.ok(&["todos", "toggle", "--help"]);
    assert!(String::from_utf8_lossy(&help.stdout).contains("inspect"));
}

#[test]
fn query_inspect_identify_and_checked_mutations_contract() {
    let s = Sandbox::new();
    let before = s.note();
    let todos = s.query();
    let first = &todos[0];
    for key in [
        "sourceID",
        "noteID",
        "title",
        "isDone",
        "metadata",
        "lineNumber",
        "sourceRevision",
        "stableId",
        "identityKind",
    ] {
        assert!(first.get(key).is_some(), "missing {key}");
    }
    assert!(first["stableId"].is_null());
    assert_eq!(first["identityKind"], "positional");
    assert_eq!(first["sourceRevision"].as_str().unwrap().len(), 71);
    assert_eq!(s.note(), before);
    let id = first["id"].as_str().unwrap();
    let rev = first["sourceRevision"].as_str().unwrap();
    let inspect: Value =
        serde_json::from_slice(&s.ok(&["todos", "inspect", "--id", id]).stdout).unwrap();
    assert_eq!(inspect["sourceRevision"], first["sourceRevision"]);
    let marked: Value = serde_json::from_slice(
        &s.ok(&["todos", "identify", "--id", id, "--expected-revision", rev])
            .stdout,
    )
    .unwrap();
    assert_eq!(marked["identityKind"], "stable");
    let stable = marked["id"].as_str().unwrap();
    assert!(stable.starts_with("task:"));
    assert_eq!(stable.len(), 37);
    let marked_bytes = s.note();
    s.ok(&[
        "todos",
        "identify",
        "--id",
        stable,
        "--expected-revision",
        marked["sourceRevision"].as_str().unwrap(),
    ]);
    assert_eq!(s.note(), marked_bytes);
    let stale = s.run(&[
        "todos",
        "toggle",
        "--id",
        stable,
        "--expected-revision",
        rev,
    ]);
    assert!(!stale.status.success());
    assert_eq!(s.note(), marked_bytes);
    s.ok(&[
        "todos",
        "update",
        "--id",
        stable,
        "--expected-revision",
        marked["sourceRevision"].as_str().unwrap(),
        "--title",
        "Renamed",
    ]);
    let current = s.query();
    assert_eq!(current[0]["id"], stable);
    assert_eq!(current[0]["title"], "Renamed");
    s.ok(&[
        "todos",
        "toggle",
        "--id",
        stable,
        "--expected-revision",
        current[0]["sourceRevision"].as_str().unwrap(),
    ]);
    assert_eq!(s.query()[0]["isDone"], true);
}

#[test]
fn refresh_discard_is_explicit_projection_only_and_blocks_pending_direct_writes() {
    let s = Sandbox::new();
    s.ok(&["todos", "sync"]);
    let first = s.query().remove(0);
    let dashboard = String::from_utf8(s.dashboard())
        .unwrap()
        .replacen("- [ ]", "- [x]", 1);
    fs::write(s.root.join("notes/dashboard.md"), dashboard).unwrap();
    let before = (s.note(), s.dashboard());
    assert!(!s.run(&["todos", "refresh"]).status.success());
    assert!(
        !s.run(&[
            "todos",
            "toggle",
            "--id",
            first["id"].as_str().unwrap(),
            "--expected-revision",
            first["sourceRevision"].as_str().unwrap()
        ])
        .status
        .success()
    );
    assert_eq!((s.note(), s.dashboard()), before);
    s.ok(&["todos", "refresh", "--discard-dashboard-edits"]);
    assert_eq!(s.note(), before.0);
    fs::write(s.root.join("notes/dashboard.md"), "legacy").unwrap();
    assert!(!s.run(&["todos", "sync"]).status.success());
    s.ok(&["todos", "refresh", "--discard-dashboard-edits"]);
    assert_eq!(s.note(), before.0);
    assert!(
        String::from_utf8_lossy(&s.ok(&["todos", "refresh", "--help"]).stdout)
            .contains("--discard-dashboard-edits")
    );
}

#[test]
fn archive_refuses_before_any_checkbox_writeback() {
    let s = Sandbox::new();
    s.ok(&["todos", "sync"]);
    fs::write(
        s.root.join("notes/dashboard.md"),
        String::from_utf8(s.dashboard())
            .unwrap()
            .replace("- [ ]", "- [x]"),
    )
    .unwrap();
    let before = (s.note(), s.dashboard());
    let output = s.run(&["todos", "archive"]);
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("subcommand"));
    assert_eq!((s.note(), s.dashboard()), before);
}

#[test]
fn query_and_tui_propagate_task_scan_errors_instead_of_empty_lists() {
    let s = Sandbox::new();
    fs::write(s.root.join("notes/duplicate.md"), s.note()).unwrap();
    let query = s.run(&["query", "todos"]);
    assert!(!query.status.success());
    assert!(String::from_utf8_lossy(&query.stderr).contains("duplicate active source"));
    let tui = s.run(&["tui", "todos"]);
    assert!(!tui.status.success());
    assert!(String::from_utf8_lossy(&tui.stderr).contains("duplicate active source"));
}

#[test]
fn sync_capability_fence_is_accepted_without_relaxing_safety() {
    let s = Sandbox::new();
    let before = s.note();
    s.ok(&["todos", "sync", "--require-task-safety-v1"]);
    assert_eq!(s.note(), before);
    let canonical = s.dashboard();
    s.ok(&["todos", "sync"]);
    assert_eq!(s.dashboard(), canonical);
    let edited = String::from_utf8(canonical)
        .unwrap()
        .replacen("- [ ]", "- [x]", 1)
        + "Personal prose\n";
    fs::write(s.root.join("notes/dashboard.md"), edited).unwrap();
    let before = (s.note(), s.dashboard());
    for args in [
        vec!["todos", "sync", "--require-task-safety-v1"],
        vec!["todos", "sync"],
    ] {
        let result = s.run(&args);
        assert!(!result.status.success());
        assert!(String::from_utf8_lossy(&result.stderr).contains("body edit"));
        assert_eq!((s.note(), s.dashboard()), before);
    }
    let help = s.ok(&["todos", "sync", "--help"]);
    let help = String::from_utf8_lossy(&help.stdout);
    assert!(help.contains("--require-task-safety-v1"));
    assert!(help.contains("Not a force override"));
}

#[test]
fn sync_capability_fence_and_malformed_extra_arguments_never_write() {
    let s = Sandbox::new();
    s.ok(&["todos", "sync", "--require-task-safety-v1"]);
    fs::write(
        s.root.join("notes/dashboard.md"),
        String::from_utf8(s.dashboard())
            .unwrap()
            .replacen("- [ ]", "- [x]", 1),
    )
    .unwrap();
    let before = (s.note(), s.dashboard());
    for args in [
        vec!["todos", "sync", "--require-task-safety-v1", "--unknown"],
        vec!["todos", "sync", "--require-task-safety-v1=false"],
        vec!["todos", "sync", "--require-task-safety-v1", "extra"],
        vec!["todos", "refresh", "--require-task-safety-v1"],
    ] {
        assert!(!s.run(&args).status.success());
        assert_eq!((s.note(), s.dashboard()), before);
    }
    s.ok(&["todos", "sync", "--require-task-safety-v1"]);
    assert!(String::from_utf8(s.note()).unwrap().contains("- [x] One"));
}
