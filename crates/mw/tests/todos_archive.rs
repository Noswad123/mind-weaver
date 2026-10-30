use serde_json::{Value, json};
use std::{
    collections::BTreeMap,
    fs,
    io::Write,
    path::{Path, PathBuf},
    process::{Command, Output, Stdio},
};

struct Fixture {
    root: PathBuf,
}
impl Fixture {
    fn new() -> Self {
        use std::sync::atomic::{AtomicUsize, Ordering};
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let root = fs::canonicalize(std::env::temp_dir())
            .unwrap()
            .join(format!(
                "mw-archive-cli-{}-{}",
                std::process::id(),
                NEXT.fetch_add(1, Ordering::Relaxed)
            ));
        fs::create_dir(&root).unwrap();
        fs::create_dir(root.join("notes")).unwrap();
        fs::create_dir(root.join("home")).unwrap();
        fs::write(
            root.join("config.toml"),
            "notes_dir = notes\ndashboard_path = notes/dashboard.md\ndb_path = missing/index.db\n",
        )
        .unwrap();
        fs::write(root.join("notes/tasks.md"), "---\nid: source\ndomains: [task]\ntask_active: true\n---\n## Todo\n### Do Now\n- [x] Done\n  arbitrary children\n- [ ] Keep\n").unwrap();
        Self { root }
    }
    fn run(&self, args: &[&str], input: Option<&Value>) -> Output {
        let mut child = Command::new(env!("CARGO_BIN_EXE_mw"))
            .args(args)
            .env_clear()
            .env("HOME", self.root.join("home"))
            .env("MW_CONFIG", self.root.join("config.toml"))
            .env("XDG_CONFIG_HOME", self.root.join("home/config"))
            .env("XDG_DATA_HOME", self.root.join("home/data"))
            .current_dir(&self.root)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .unwrap();
        if let Some(input) = input {
            child
                .stdin
                .take()
                .unwrap()
                .write_all(&serde_json::to_vec(input).unwrap())
                .unwrap();
        } else {
            drop(child.stdin.take());
        }
        child.wait_with_output().unwrap()
    }
    fn ok(&self, args: &[&str], input: Option<&Value>) -> Value {
        let out = self.run(args, input);
        assert!(
            out.status.success(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        serde_json::from_slice(&out.stdout).unwrap()
    }
    fn plan(&self) -> Value {
        let s = self.ok(&["todos", "snapshot"], None);
        let t = &s["items"][0];
        self.ok(
            &[
                "todos",
                "archive",
                "plan",
                "--id",
                t["id"].as_str().unwrap(),
                "--expected-revision",
                t["sourceRevision"].as_str().unwrap(),
                "--archive-path",
                "ledger.md",
                "--json",
            ],
            None,
        )
    }
    fn inspect(&self, p: &Value) -> Value {
        self.ok(
            &[
                "todos",
                "archive",
                "inspect",
                "--archive-path",
                "ledger.md",
                "--operation-id",
                p["operationId"].as_str().unwrap(),
                "--json",
            ],
            None,
        )
    }
}
impl Drop for Fixture {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root).unwrap();
    }
}
fn tree(root: &Path) -> BTreeMap<PathBuf, Option<Vec<u8>>> {
    let mut out = BTreeMap::new();
    for entry in fs::read_dir(root).unwrap() {
        let p = entry.unwrap().path();
        if p.is_dir() {
            out.insert(p.clone(), None);
            out.extend(tree(&p));
        } else {
            out.insert(p.clone(), Some(fs::read(&p).unwrap()));
        }
    }
    out
}

#[test]
fn plan_inspect_are_pure_and_roundtrip_dtos_match_contract() {
    let f = Fixture::new();
    let before = tree(&f.root);
    let p = f.plan();
    assert_eq!(tree(&f.root), before);
    assert_eq!(p.as_object().unwrap().len(), 15);
    assert_eq!(p["kind"], "task-archive-plan");
    assert_eq!(p["schemaVersion"], "1");
    assert_eq!(p["archiveRevision"], "absent");
    assert_eq!(p["dashboardRevision"], "absent");
    assert_eq!(p["taskCount"], 1);
    assert_eq!(
        p["rawByteCount"],
        "- [x] Done\n  arbitrary children\n".len()
    );
    assert!(serde_json::from_value::<mw_notes::ArchivePlan>(p.clone()).is_ok());
    let r = f.ok(&["todos", "archive", "commit", "--plan", "-"], Some(&p));
    assert_eq!(r["kind"], "task-archive-result");
    assert_eq!(r["phase"], "completed");
    assert_eq!(r["sourcePruned"], true);
    let before = tree(&f.root);
    let i = f.inspect(&p);
    assert_eq!(tree(&f.root), before);
    assert_eq!(i["plan"], p);
    assert_eq!(i["sourceState"], "post");
    assert_eq!(i["nextAction"], "none");
    assert_eq!(i["blockedReason"], Value::Null);
    assert_eq!(i["projectionStatus"], "current");
    let r = f.ok(
        &["todos", "archive", "recover", "--inspection", "-"],
        Some(&i),
    );
    assert_eq!(r["alreadyComplete"], true);
    assert_eq!(r["sourcePruned"], false);
    assert_eq!(r["projectionUpdated"], false);
    assert_eq!(tree(&f.root), before);
    assert!(!f.root.join("missing").exists());
}

#[test]
fn missing_arguments_bad_json_and_versions_do_not_mutate() {
    let f = Fixture::new();
    let p = f.plan();
    let before = tree(&f.root);
    for args in [
        vec!["todos", "archive"],
        vec!["todos", "archive", "plan"],
        vec!["todos", "archive", "commit"],
        vec!["todos", "archive", "recover"],
        vec!["todos", "archive", "inspect"],
        vec!["todos", "archive", "commit", "--plan", "file.json"],
    ] {
        assert!(!f.run(&args, None).status.success());
        assert_eq!(tree(&f.root), before);
    }
    for input in [
        json!(null),
        json!({"schemaVersion":"2"}),
        {
            let mut p = p.clone();
            p["schemaVersion"] = "2".into();
            p
        },
        {
            let mut p = p.clone();
            p["extra"] = true.into();
            p
        },
    ] {
        assert!(
            !f.run(&["todos", "archive", "commit", "--plan", "-"], Some(&input))
                .status
                .success()
        );
        assert_eq!(tree(&f.root), before);
    }
    assert!(
        !f.run(&["todos", "archive", "commit", "--plan", "-"], None)
            .status
            .success()
    );
    assert_eq!(tree(&f.root), before);
}

#[test]
fn configured_profile_mismatch_and_stale_plan_are_refused() {
    let f = Fixture::new();
    let p = f.plan();
    fs::write(
        f.root.join("config.toml"),
        "notes_dir = notes\ndashboard_path = other-dashboard.md\ndb_path = missing/index.db\n",
    )
    .unwrap();
    let before = tree(&f.root);
    assert!(
        !f.run(&["todos", "archive", "commit", "--plan", "-"], Some(&p))
            .status
            .success()
    );
    assert_eq!(tree(&f.root), before);
    fs::write(
        f.root.join("config.toml"),
        "notes_dir = notes\ndashboard_path = notes/dashboard.md\ndb_path = missing/index.db\n",
    )
    .unwrap();
    fs::write(f.root.join("ledger.md"), "existing month note").unwrap();
    let before = tree(&f.root);
    assert!(
        !f.run(&["todos", "archive", "commit", "--plan", "-"], Some(&p))
            .status
            .success()
    );
    // Lock directories may be created by commit, but all user bytes remain unchanged.
    for (path, bytes) in before {
        if let Some(bytes) = bytes {
            assert_eq!(fs::read(path).unwrap(), bytes);
        }
    }
    assert!(!f.root.join("missing").exists());
}

#[test]
fn fully_parameterized_pending_dashboard_plan_refuses_without_writeback() {
    let f = Fixture::new();
    let s = f.ok(&["todos", "snapshot"], None);
    let t = &s["items"][0];
    let out = f.run(&["todos", "refresh"], None);
    assert!(out.status.success());
    let dashboard = f.root.join("notes/dashboard.md");
    fs::write(
        &dashboard,
        fs::read_to_string(&dashboard)
            .unwrap()
            .replace("- [ ] Keep", "- [x] Keep"),
    )
    .unwrap();
    let before = tree(&f.root);
    let output = f.run(
        &[
            "todos",
            "archive",
            "plan",
            "--id",
            t["id"].as_str().unwrap(),
            "--expected-revision",
            t["sourceRevision"].as_str().unwrap(),
            "--archive-path",
            "ledger.md",
        ],
        None,
    );
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("pending dashboard"));
    assert_eq!(tree(&f.root), before);
}

#[test]
fn literal_relative_paths_do_not_expand_environment_or_tilde() {
    let f = Fixture::new();
    let s = f.ok(&["todos", "snapshot"], None);
    let t = &s["items"][0];
    for name in ["$HOME.md", "~.md", "ledger.md"] {
        let p = f.ok(
            &[
                "todos",
                "archive",
                "plan",
                "--id",
                t["id"].as_str().unwrap(),
                "--expected-revision",
                t["sourceRevision"].as_str().unwrap(),
                "--archive-path",
                name,
            ],
            None,
        );
        assert_eq!(p["archivePath"], f.root.join(name).to_str().unwrap());
    }
}

#[test]
fn precision_float_context_survives_cli_plan_and_both_stdin_protocols() {
    let f = Fixture::new();
    let source = f.root.join("notes/tasks.md");
    fs::write(
        &source,
        fs::read_to_string(&source)
            .unwrap()
            .replace("- [x] Done", "- [x] Precision w:51.248178375505404"),
    )
    .unwrap();
    let expected = 51.248178375505404_f64;
    let plan = f.plan();
    assert_eq!(
        plan["items"][0]["task"]["metadata"]["effectiveWeight"]
            .as_f64()
            .unwrap()
            .to_bits(),
        expected.to_bits()
    );
    let typed: mw_notes::ArchivePlan = serde_json::from_value(plan.clone()).unwrap();
    assert_eq!(
        typed.items[0].task.metadata.effective_weight.to_bits(),
        expected.to_bits()
    );
    assert!(
        serde_json::to_string(&plan)
            .unwrap()
            .contains("\"effectiveWeight\":51.248178375505404")
    );
    f.ok(&["todos", "archive", "commit", "--plan", "-"], Some(&plan));
    let inspection = f.inspect(&plan);
    assert_eq!(inspection["plan"], plan);
    assert_eq!(
        inspection["plan"]["items"][0]["task"]["metadata"]["effectiveWeight"]
            .as_f64()
            .unwrap()
            .to_bits(),
        expected.to_bits()
    );
    // Require a real recovery publication, not just the completed no-op path.
    fs::remove_file(f.root.join("notes/dashboard.md")).unwrap();
    let inspection = f.inspect(&plan);
    assert_eq!(inspection["nextAction"], "refreshOnly");
    let result = f.ok(
        &["todos", "archive", "recover", "--inspection", "-"],
        Some(&inspection),
    );
    assert_eq!(result["projectionUpdated"], true);
    assert_eq!(result["sourcePruned"], false);
    let completed = f.inspect(&plan);
    assert_eq!(completed["plan"], plan);
    assert_eq!(
        completed["plan"]["items"][0]["task"]["metadata"]["effectiveWeight"]
            .as_f64()
            .unwrap()
            .to_bits(),
        expected.to_bits()
    );
}

#[test]
fn format_seal_and_index_maintenance_preserve_inside_root_archives_including_hubs() {
    for destination in ["notes/archive.md", "notes/hub.md", "notes/area/hub.md"] {
        let f = Fixture::new();
        if destination.contains("/area/") {
            fs::create_dir(f.root.join("notes/area")).unwrap();
        }
        let source = f.root.join("notes/tasks.md");
        fs::write(&source, "---\nid: source\ndomains: [task]\ntask_active: true\n---\n## Todo\n### Do Now\n- [x] Selected\r\n  ```markdown\r\n# Raw example heading\n- [ ] Example only\n  ```\r\n  child\tbytes\r\n- [x] Next archive\n- [ ] Keep\n").unwrap();
        let snapshot = f.ok(&["todos", "snapshot"], None);
        let task = &snapshot["items"][0];
        let plan = f.ok(
            &[
                "todos",
                "archive",
                "plan",
                "--id",
                task["id"].as_str().unwrap(),
                "--expected-revision",
                task["sourceRevision"].as_str().unwrap(),
                "--archive-path",
                destination,
            ],
            None,
        );
        f.ok(&["todos", "archive", "commit", "--plan", "-"], Some(&plan));
        let path = f.root.join(destination);
        let original = fs::read(&path).unwrap();
        for args in [
            vec!["notes", "format", "--all"],
            vec!["notes", "format"],
            vec!["seal"],
            vec!["notes", "seal"],
            vec!["notes", "ingest", "--prune"],
            vec!["notes", "register"],
            vec!["notes", "validate-registry"],
        ] {
            let output = f.run(&args, None);
            assert!(
                output.status.success(),
                "{destination} {args:?}: {}",
                String::from_utf8_lossy(&output.stderr)
            );
            assert_eq!(fs::read(&path).unwrap(), original, "{destination} {args:?}");
            let inspection = mw_notes::inspect_archive(
                f.root.join("notes"),
                f.root.join("notes/dashboard.md"),
                &path,
                plan["operationId"].as_str().unwrap(),
            )
            .unwrap();
            assert_eq!(inspection.phase, "completed");
            assert_eq!(inspection.source_state, "changed");
        }
        assert!(f.root.join("missing/index.db").is_file());
        let registry = mw_notes::build_registry(f.root.join("notes")).unwrap();
        let entry = registry
            .registry
            .entries
            .values()
            .find(|e| e.path == destination.strip_prefix("notes/").unwrap())
            .unwrap();
        assert!(entry.id.starts_with("mw-archive-"));
        let indexed = f.ok(&["query", "ids", "--uid", &entry.id], None);
        assert_eq!(indexed["status"], "resolved");
        let snapshot = f.ok(&["todos", "snapshot"], None);
        let remaining = &snapshot["items"][0];
        assert_eq!(remaining["done"], true);
        let next = f.ok(
            &[
                "todos",
                "archive",
                "plan",
                "--id",
                remaining["id"].as_str().unwrap(),
                "--expected-revision",
                remaining["sourceRevision"].as_str().unwrap(),
                "--archive-path",
                destination,
            ],
            None,
        );
        assert_ne!(next["sourceRevision"], plan["postSourceRevision"]);
        assert_ne!(next["archiveRevision"], "absent");
        assert_eq!(fs::read(&path).unwrap(), original);
    }
}
