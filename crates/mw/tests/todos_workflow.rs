use serde_json::Value;
use std::{
    fs,
    io::Write,
    path::PathBuf,
    process::{Command, Output, Stdio},
    sync::atomic::{AtomicUsize, Ordering},
};

const ID: &str = "0123456789abcdef0123456789abcdef";
const LEGACY_SOURCE: &str = "---\nid: project\ndomains: [task-index]\ntask_active: true\ntask_scope: project\ntask_area: Action\n---\n## Todo\n### Next\n- [ ] Work\n### Inbox\n### Waiting\n";
const LEGACY_DASHBOARD: &str = "---\nid: dashboard\ntags: [preserve]\n---\n# Dashboard\n\n## Code\n\n## Action\n- [ ] Work [[project]]\n\n## Reading\n\n## Amusement\n\n## Music\n\n## Exercise\n\n## Love\n";

struct Sandbox(PathBuf);

impl Sandbox {
    fn new() -> Self {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let root = fs::canonicalize(std::env::temp_dir())
            .unwrap()
            .join(format!(
                "mw-workflow-{}-{}",
                std::process::id(),
                NEXT.fetch_add(1, Ordering::Relaxed)
            ));
        fs::create_dir(&root).unwrap();
        fs::create_dir(root.join("notes")).unwrap();
        fs::write(root.join("config.toml"), "notes_dir = notes\ninbox_path = notes/capture.md\ndashboard_path = notes/dashboard.md\ndb_path = unavailable/db.sqlite\nnotes_schema_path = missing/schema.sql\n").unwrap();
        Self(root)
    }

    fn run(&self, args: &[&str], input: Option<&[u8]>) -> Output {
        let mut command = Command::new(env!("CARGO_BIN_EXE_mw"));
        command
            .args(args)
            .env_clear()
            .env("HOME", self.0.join("home"))
            .env("MW_CONFIG", self.0.join("config.toml"))
            .env("XDG_CONFIG_HOME", self.0.join("config"))
            .env("XDG_DATA_HOME", self.0.join("data"))
            .current_dir(&self.0)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .stdin(Stdio::piped());
        let mut child = command.spawn().unwrap();
        if let Some(input) = input {
            child.stdin.take().unwrap().write_all(input).unwrap();
        } else {
            drop(child.stdin.take());
        }
        child.wait_with_output().unwrap()
    }

    fn ok(&self, args: &[&str], input: Option<&[u8]>) -> Output {
        let output = self.run(args, input);
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        output
    }

    fn read(&self, relative: &str) -> Vec<u8> {
        fs::read(self.0.join(relative)).unwrap()
    }

    fn snapshot(&self) -> Value {
        serde_json::from_slice(&self.ok(&["todos", "snapshot", "--json"], None).stdout).unwrap()
    }
}

impl Drop for Sandbox {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

#[test]
fn capture_uses_configured_inbox_without_database_and_retries_once() {
    let s = Sandbox::new();
    let args = ["todos", "add", "Call dentist", "--capture-id", ID, "--json"];
    let first: Value = serde_json::from_slice(&s.ok(&args, None).stdout).unwrap();
    assert_eq!(first["schemaVersion"], "1");
    assert_eq!(first["alreadyCaptured"], false);
    assert_eq!(first["task"]["sourcePath"], "capture.md");
    assert_eq!(first["task"]["id"], format!("task:{ID}"));
    assert_eq!(first["task"]["todoSection"], "Inbox");
    let source = s.read("notes/capture.md");
    let dashboard = s.read("notes/dashboard.md");
    let repeated: Value = serde_json::from_slice(&s.ok(&args, None).stdout).unwrap();
    assert_eq!(repeated["alreadyCaptured"], true);
    assert_eq!(repeated["projectionUpdated"], false);
    assert_eq!(s.read("notes/capture.md"), source);
    assert_eq!(s.read("notes/dashboard.md"), dashboard);
    assert_eq!(s.snapshot()["items"].as_array().unwrap().len(), 1);
    assert!(!s.0.join("unavailable").exists());
    assert!(!s.0.join("data").exists());
    let human = s.ok(&["todos", "add", "Another task"], None);
    assert!(
        String::from_utf8(human.stdout)
            .unwrap()
            .contains("captured: Another task")
    );
}

#[test]
fn completion_is_explicit_repeatable_and_archival_remains_a_separate_checked_action() {
    let s = Sandbox::new();
    let created: Value = serde_json::from_slice(
        &s.ok(
            &[
                "todos",
                "add",
                "Accomplishment",
                "--capture-id",
                ID,
                "--json",
            ],
            None,
        )
        .stdout,
    )
    .unwrap();
    let task = &created["task"];
    let id = task["id"].as_str().unwrap();
    let initial_revision = task["sourceRevision"].as_str().unwrap();
    let done: Value = serde_json::from_slice(
        &s.ok(
            &[
                "todos",
                "done",
                "--id",
                id,
                "--expected-revision",
                initial_revision,
                "--json",
            ],
            None,
        )
        .stdout,
    )
    .unwrap();
    assert_eq!(done["done"], true);
    assert_eq!(done["id"], id);
    let completed_revision = done["sourceRevision"].as_str().unwrap();
    assert_ne!(initial_revision, completed_revision);
    let source = s.read("notes/capture.md");
    let dashboard = s.read("notes/dashboard.md");
    let modified = fs::metadata(s.0.join("notes/capture.md"))
        .unwrap()
        .modified()
        .unwrap();
    let repeated: Value = serde_json::from_slice(
        &s.ok(
            &[
                "todos",
                "done",
                "--id",
                id,
                "--expected-revision",
                completed_revision,
                "--json",
            ],
            None,
        )
        .stdout,
    )
    .unwrap();
    assert_eq!(repeated, done);
    assert_eq!(
        fs::metadata(s.0.join("notes/capture.md"))
            .unwrap()
            .modified()
            .unwrap(),
        modified
    );
    assert_eq!(s.read("notes/dashboard.md"), dashboard);
    assert_eq!(s.read("notes/capture.md"), source);
    assert!(
        !s.run(
            &[
                "todos",
                "done",
                "--id",
                id,
                "--expected-revision",
                initial_revision
            ],
            None
        )
        .status
        .success()
    );
    assert!(!s.0.join("notes/life-log.md").exists());
    let reopened: Value = serde_json::from_slice(
        &s.ok(
            &[
                "todos",
                "reopen",
                "--id",
                id,
                "--expected-revision",
                completed_revision,
                "--json",
            ],
            None,
        )
        .stdout,
    )
    .unwrap();
    assert_eq!(reopened["done"], false);
    let done: Value = serde_json::from_slice(
        &s.ok(
            &[
                "todos",
                "done",
                "--id",
                id,
                "--expected-revision",
                reopened["sourceRevision"].as_str().unwrap(),
                "--json",
            ],
            None,
        )
        .stdout,
    )
    .unwrap();
    let plan = s
        .ok(
            &[
                "todos",
                "archive",
                "plan",
                "--id",
                id,
                "--expected-revision",
                done["sourceRevision"].as_str().unwrap(),
                "--archive-path",
                "notes/life-log.md",
            ],
            None,
        )
        .stdout;
    s.ok(&["todos", "archive", "commit", "--plan", "-"], Some(&plan));
    assert!(s.snapshot()["items"].as_array().unwrap().is_empty());
    assert!(
        String::from_utf8(s.read("notes/life-log.md"))
            .unwrap()
            .contains("Accomplishment")
    );
    let retry = s.run(
        &[
            "todos",
            "add",
            "Accomplishment",
            "--capture-id",
            ID,
            "--json",
        ],
        None,
    );
    assert!(!retry.status.success());
    assert!(String::from_utf8_lossy(&retry.stderr).contains("already archived"));
}

#[test]
fn already_done_is_still_guarded_by_dashboard_preflight() {
    let s = Sandbox::new();
    fs::write(s.0.join("notes/tasks.md"), "---\nid: tasks\ndomains: [task]\ntask_active: true\n---\n## Todo\n### Do Now\n- [x] Finished\n  <!-- mw-task:v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n- [ ] Pending\n").unwrap();
    s.ok(&["todos", "refresh"], None);
    let snapshot = s.snapshot();
    let task = &snapshot["items"][0];
    let dashboard = String::from_utf8(s.read("notes/dashboard.md"))
        .unwrap()
        .replace("- [ ] Pending", "- [x] Pending");
    fs::write(s.0.join("notes/dashboard.md"), &dashboard).unwrap();
    let before = s.read("notes/tasks.md");
    let output = s.run(
        &[
            "todos",
            "done",
            "--id",
            task["id"].as_str().unwrap(),
            "--expected-revision",
            task["sourceRevision"].as_str().unwrap(),
        ],
        None,
    );
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("pending dashboard"));
    assert_eq!(s.read("notes/tasks.md"), before);
    assert_eq!(s.read("notes/dashboard.md"), dashboard.as_bytes());
}

#[test]
fn migration_is_explicit_backed_up_and_leaves_completed_tasks_for_later_archive() {
    let s = Sandbox::new();
    let original = "---\nid: project\ndomains: [task-index]\ntask_active: true\ntask_scope: project\n---\n## Todo\n### Next\n- [x] Accomplishment\n### Inbox\n- [ ] Idea\n### Waiting\n- [ ] Dependency\n";
    fs::write(s.0.join("notes/project.md"), original).unwrap();
    assert!(!s.run(&["todos", "snapshot"], None).status.success());
    let plan = s.ok(&["todos", "migrate", "plan", "--json"], None).stdout;
    let preview: Value = serde_json::from_slice(&plan).unwrap();
    assert_eq!(preview["changes"].as_array().unwrap().len(), 1);
    assert!(preview["blockers"].as_array().unwrap().is_empty());
    assert_eq!(s.read("notes/project.md"), original.as_bytes());
    assert!(!s.0.join("notes/.mw").exists());
    assert!(!s.0.join("notes/dashboard.md").exists());
    let result = s.ok(&["todos", "migrate", "commit", "--plan", "-"], Some(&plan));
    let result: Value = serde_json::from_slice(&result.stdout).unwrap();
    let backup = PathBuf::from(result["backupPath"].as_str().unwrap());
    assert_eq!(
        fs::read(backup.join("source-0.bak")).unwrap(),
        original.as_bytes()
    );
    let updated = String::from_utf8(s.read("notes/project.md")).unwrap();
    assert_eq!(
        updated,
        original
            .replace("task-index", "task")
            .replace("### Next", "### Do Now")
            .replace("### Waiting", "### Blocked")
    );
    let snapshot = s.snapshot();
    assert_eq!(snapshot["items"].as_array().unwrap().len(), 3);
    assert_eq!(snapshot["items"][0]["done"], true);
    assert!(
        String::from_utf8(s.read("notes/dashboard.md"))
            .unwrap()
            .contains("- [x] Accomplishment")
    );
    assert!(
        !s.run(&["todos", "migrate", "commit", "--plan", "-"], Some(&plan))
            .status
            .success()
    );
    assert!(!s.0.join("unavailable").exists());
}

#[test]
fn legacy_dashboard_upgrade_requires_checked_opt_in_and_preserves_backups() {
    let s = Sandbox::new();
    fs::write(s.0.join("notes/project.md"), LEGACY_SOURCE).unwrap();
    fs::write(s.0.join("notes/dashboard.md"), LEGACY_DASHBOARD).unwrap();
    let blocked: Value =
        serde_json::from_slice(&s.ok(&["todos", "migrate", "plan", "--json"], None).stdout)
            .unwrap();
    assert_eq!(blocked["bootstrapLegacyDashboard"], false);
    assert!(!blocked["blockers"].as_array().unwrap().is_empty());
    let plan = s
        .ok(
            &[
                "todos",
                "migrate",
                "plan",
                "--bootstrap-legacy-dashboard",
                "--json",
            ],
            None,
        )
        .stdout;
    let preview: Value = serde_json::from_slice(&plan).unwrap();
    assert_eq!(preview["bootstrapLegacyDashboard"], true);
    assert!(preview["blockers"].as_array().unwrap().is_empty());
    assert_eq!(s.read("notes/project.md"), LEGACY_SOURCE.as_bytes());
    assert_eq!(s.read("notes/dashboard.md"), LEGACY_DASHBOARD.as_bytes());
    assert!(!s.0.join("notes/.mw").exists());
    let output = s.ok(&["todos", "migrate", "commit", "--plan", "-"], Some(&plan));
    let result: Value = serde_json::from_slice(&output.stdout).unwrap();
    let backup = PathBuf::from(result["backupPath"].as_str().unwrap());
    assert_eq!(
        fs::read(backup.join("dashboard.bak")).unwrap(),
        LEGACY_DASHBOARD.as_bytes()
    );
    assert_eq!(
        fs::read(backup.join("source-0.bak")).unwrap(),
        LEGACY_SOURCE.as_bytes()
    );
    let dashboard = String::from_utf8(s.read("notes/dashboard.md")).unwrap();
    assert!(dashboard.starts_with("---\nid: dashboard\ntags: [preserve]\n---\n"));
    assert!(dashboard.contains("<!-- mw-dashboard:v1 -->"));
    assert!(dashboard.contains("- [ ] Work <!-- mw-dashboard:v1:"));
    assert_eq!(s.snapshot()["items"][0]["todoSection"], "Do Now");
    s.ok(&["todos", "refresh"], None);
    assert!(!s.0.join("unavailable").exists());
}

#[test]
fn legacy_bootstrap_never_discards_checkbox_edits_or_downgrades_protocols() {
    for dashboard in [
        LEGACY_DASHBOARD.replace("- [ ]", "- [x]"),
        LEGACY_DASHBOARD.replace("Work [[project]]", "Changed text [[project]]"),
        LEGACY_DASHBOARD.replace("# Dashboard", "<!-- mw-dashboard:v2 -->\n# Dashboard"),
    ] {
        let s = Sandbox::new();
        fs::write(s.0.join("notes/project.md"), LEGACY_SOURCE).unwrap();
        fs::write(s.0.join("notes/dashboard.md"), &dashboard).unwrap();
        let plan = s
            .ok(
                &["todos", "migrate", "plan", "--bootstrap-legacy-dashboard"],
                None,
            )
            .stdout;
        let preview: Value = serde_json::from_slice(&plan).unwrap();
        assert!(!preview["blockers"].as_array().unwrap().is_empty());
        assert!(
            !s.run(&["todos", "migrate", "commit", "--plan", "-"], Some(&plan))
                .status
                .success()
        );
        assert_eq!(s.read("notes/project.md"), LEGACY_SOURCE.as_bytes());
        assert_eq!(s.read("notes/dashboard.md"), dashboard.as_bytes());
    }
}

#[test]
fn legacy_upgrade_binds_dashboard_bytes_and_the_reviewed_opt_in() {
    let s = Sandbox::new();
    fs::write(s.0.join("notes/project.md"), LEGACY_SOURCE).unwrap();
    fs::write(s.0.join("notes/dashboard.md"), LEGACY_DASHBOARD).unwrap();
    let plan = s
        .ok(
            &["todos", "migrate", "plan", "--bootstrap-legacy-dashboard"],
            None,
        )
        .stdout;
    let mut tampered: Value = serde_json::from_slice(&plan).unwrap();
    tampered["bootstrapLegacyDashboard"] = false.into();
    let tampered = serde_json::to_vec(&tampered).unwrap();
    assert!(
        !s.run(
            &["todos", "migrate", "commit", "--plan", "-"],
            Some(&tampered)
        )
        .status
        .success()
    );
    let external = format!("{LEGACY_DASHBOARD}\n");
    fs::write(s.0.join("notes/dashboard.md"), &external).unwrap();
    assert!(
        !s.run(&["todos", "migrate", "commit", "--plan", "-"], Some(&plan))
            .status
            .success()
    );
    assert_eq!(s.read("notes/project.md"), LEGACY_SOURCE.as_bytes());
    assert_eq!(s.read("notes/dashboard.md"), external.as_bytes());
}

#[test]
fn malformed_stale_and_tampered_plans_never_change_sources() {
    let s = Sandbox::new();
    let original = "---\nid: old\ndomains: [task-index]\ntask_active: true\n---\n## Todo\n### Next\n- [ ] Work\n";
    fs::write(s.0.join("notes/old.md"), original).unwrap();
    let plan = s.ok(&["todos", "migrate", "plan"], None).stdout;
    let mut tampered: Value = serde_json::from_slice(&plan).unwrap();
    tampered["changes"][0]["replacement"] = "overwrite attempt".into();
    for input in [
        b"{}".to_vec(),
        serde_json::to_vec(&tampered).unwrap(),
        [plan.clone(), b" {}".to_vec()].concat(),
    ] {
        assert!(
            !s.run(&["todos", "migrate", "commit", "--plan", "-"], Some(&input))
                .status
                .success()
        );
        assert_eq!(s.read("notes/old.md"), original.as_bytes());
        assert!(!s.0.join("notes/dashboard.md").exists());
    }
    fs::write(
        s.0.join("notes/old.md"),
        format!("{original}\nexternal edit\n"),
    )
    .unwrap();
    let external = s.read("notes/old.md");
    assert!(
        !s.run(&["todos", "migrate", "commit", "--plan", "-"], Some(&plan))
            .status
            .success()
    );
    assert_eq!(s.read("notes/old.md"), external);
}

#[test]
fn invalid_capture_and_legacy_sources_fail_before_source_creation() {
    let s = Sandbox::new();
    for text in ["", "line\nbreak", "<!-- mw-task:v1:bad -->"] {
        assert!(
            !s.run(&["todos", "add", text, "--json"], None)
                .status
                .success()
        );
        assert!(!s.0.join("notes/capture.md").exists());
    }
    fs::write(
        s.0.join("notes/old.md"),
        "---\ndomains: [task-index]\ntask_active: true\n---\n## Todo\n### Next\n- [ ] Existing\n",
    )
    .unwrap();
    let output = s.run(&["todos", "add", "New task"], None);
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("migrate"));
    assert!(!s.0.join("notes/capture.md").exists());
}

#[test]
fn task_schema_is_canonical_and_legacy_schema_lookup_is_actionable() {
    let s = Sandbox::new();
    let schemas: Value =
        serde_json::from_slice(&s.ok(&["schema", "list", "--json"], None).stdout).unwrap();
    let task = schemas["schemas"]
        .as_array()
        .unwrap()
        .iter()
        .find(|s| s["domain"] == "task")
        .unwrap();
    assert!(task["aliases"].as_array().unwrap().is_empty());
    let output = s.run(&["schema", "show", "task-index", "--json"], None);
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("migrat"));
}
