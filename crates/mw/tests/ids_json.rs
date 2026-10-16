use std::{
    collections::BTreeMap,
    fs,
    path::{Path, PathBuf},
    process::{Command, Output},
    sync::atomic::{AtomicUsize, Ordering},
};

use mw_db::NoteDb;
use serde_json::{Value, json};

struct Sandbox {
    root: PathBuf,
}

fn snapshot(root: &Path) -> BTreeMap<PathBuf, Vec<u8>> {
    let mut files = BTreeMap::new();
    for entry in fs::read_dir(root).unwrap() {
        let path = entry.unwrap().path();
        if path.is_dir() {
            files.extend(snapshot(&path));
        } else {
            files.insert(path.clone(), fs::read(path).unwrap());
        }
    }
    files
}

impl Sandbox {
    fn new() -> Self {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let root = fs::canonicalize(std::env::temp_dir())
            .unwrap()
            .join(format!(
                "mw-ids-json-{}-{}",
                std::process::id(),
                NEXT.fetch_add(1, Ordering::Relaxed)
            ));
        fs::create_dir(&root).unwrap();
        for dir in ["home", "cwd", "config"] {
            fs::create_dir(root.join(dir)).unwrap();
        }
        fs::write(
            root.join("config/config.toml"),
            "notes_dir = missing-notes\ndb_path = index.db\nnotes_schema_path = missing-schema.sql\n",
        )
        .unwrap();
        Self { root }
    }

    fn path(&self) -> PathBuf {
        self.root.join("cwd/index.db")
    }

    // Only fixture setup uses the writable opener.
    fn db(&self) -> NoteDb {
        NoteDb::open(self.path(), None).unwrap()
    }

    fn command(&self, args: &[&str]) -> Command {
        let mut cmd = Command::new(env!("CARGO_BIN_EXE_mw"));
        cmd.args(["query", "ids"])
            .args(args)
            .env_clear()
            .env("HOME", self.root.join("home"))
            .env("XDG_CONFIG_HOME", self.root.join("xdg-config"))
            .env("XDG_DATA_HOME", self.root.join("xdg-data"))
            .env("XDG_STATE_HOME", self.root.join("xdg-state"))
            .env("MW_CONFIG", self.root.join("config/config.toml"))
            .current_dir(self.root.join("cwd"));
        cmd
    }

    fn run(&self, args: &[&str]) -> Output {
        let before = snapshot(&self.root);
        let output = self.command(args).output().unwrap();
        assert_eq!(snapshot(&self.root), before, "query changed fixture files");
        assert!(!self.root.join("cwd/missing-notes").exists());
        output
    }

    fn query(&self, args: &[&str]) -> Value {
        let output = self.run(args);
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        assert!(output.stderr.is_empty());
        let value: Value = serde_json::from_slice(&output.stdout).unwrap();
        assert_eq!(value["schemaVersion"], "1");
        assert_eq!(value["source"], "indexed-registry");
        assert_eq!(
            value["notesDir"],
            self.root.join("cwd/missing-notes").to_str().unwrap()
        );
        value
    }
}

impl Drop for Sandbox {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root).unwrap();
    }
}

fn insert(db: &NoteDb, uid: &str, path: &str, title: &str) -> i64 {
    db.connection()
        .execute(
            "INSERT INTO notes(path,title,content) VALUES (?1,?2,'SECRET CONTENT')",
            [path, title],
        )
        .unwrap();
    let id = db.connection().last_insert_rowid();
    db.connection()
        .execute(
            "INSERT INTO note_ids(note_id,note_uid,path,is_hub) VALUES (?1,?2,?3,1)",
            [id.to_string().as_str(), uid, path],
        )
        .unwrap();
    id
}

fn conflict(db: &NoteDb, uid: &str, path: &str, reason: &str) {
    db.connection()
        .execute(
            "INSERT INTO note_id_conflicts(note_uid,path,reason) VALUES (?1,?2,?3)",
            [uid, path, reason],
        )
        .unwrap();
}

#[test]
fn literal_metadata_and_exact_contract_are_preserved_without_notes_tree() {
    let s = Sandbox::new();
    let uid = "  A b/\u{96ea}%_.md#tag  ";
    let path = " dir/\u{96ea} %_#file.md ";
    let title = "  Title \u{96ea} %_/#.md  ";
    let id = insert(&s.db(), uid, path, title);
    let value = s.query(&["--uid", uid]);
    assert_eq!(
        value,
        json!({
            "schemaVersion":"1", "source":"indexed-registry",
            "notesDir":s.root.join("cwd/missing-notes"), "mode":"exact", "uid":uid,
            "status":"resolved", "target":{"uid":uid,"noteID":id,"path":path,"title":title,"isHub":true}
        })
    );
    assert_eq!(s.query(&["--uid", uid.trim()])["status"], "notFound");
    let prefix = s.query(&["--prefix", "  A b/\u{96ea}%_"]);
    assert_eq!(prefix["items"], json!([value["target"]]));
    assert_eq!(prefix.as_object().unwrap().len(), 7);
    assert!(!prefix.to_string().contains("SECRET CONTENT"));
}

#[test]
fn hyphen_prefixed_literals_work_as_separate_argv_values() {
    let s = Sandbox::new();
    let db = s.db();
    for uid in ["-literal", "--option-looking", "literal", "option-looking"] {
        insert(&db, uid, &format!("{uid}.md"), uid);
    }
    drop(db);
    for uid in ["-literal", "--option-looking"] {
        let exact = s.query(&["--uid", uid]);
        assert_eq!(exact["mode"], "exact");
        assert_eq!(exact["uid"], uid);
        assert_eq!(exact["status"], "resolved");
        assert_eq!(exact["target"]["uid"], uid);
        assert_eq!(exact["target"]["path"], format!("{uid}.md"));

        let prefix = s.query(&["--prefix", uid, "--limit", "1"]);
        assert_eq!(prefix["mode"], "prefix");
        assert_eq!(prefix["prefix"], uid);
        assert_eq!(prefix["items"], json!([exact["target"]]));
        assert_eq!(prefix["hasMore"], false);
    }
}

#[test]
fn binary_case_order_and_has_more_are_bounded() {
    let s = Sandbox::new();
    let db = s.db();
    for uid in ["foo", "FoO", "Zoo", "alpha"] {
        insert(&db, uid, &format!("{uid}.md"), "");
    }
    drop(db);
    let value = s.query(&["--prefix", "", "--limit", "2"]);
    assert_eq!(value["hasMore"], true);
    assert_eq!(value["items"][0]["uid"], "FoO");
    assert_eq!(value["items"][1]["uid"], "Zoo");
    let all = s.query(&["--prefix", "", "--limit", "200"]);
    assert_eq!(all["items"].as_array().unwrap().len(), 4);
    assert_eq!(all["hasMore"], false);
    for uid in ["FoO", "foo"] {
        assert_eq!(s.query(&["--uid", uid])["target"]["uid"], uid);
    }
    assert_eq!(s.query(&["--uid", "FOO"])["status"], "notFound");
}

#[test]
fn prefix_is_literal_not_like_or_sql() {
    let s = Sandbox::new();
    let db = s.db();
    for uid in ["a%_one", "a%_two", "axone", "a_one", "' OR 1=1 --"] {
        insert(&db, uid, &format!("{uid}.md"), "title");
    }
    drop(db);
    let v = s.query(&["--prefix", "a%_", "--limit", "1"]);
    assert_eq!(v["items"].as_array().unwrap().len(), 1);
    assert_eq!(v["items"][0]["uid"], "a%_one");
    assert_eq!(v["hasMore"], true);
    assert_eq!(
        s.query(&["--prefix", "' OR 1=1 --"])["items"]
            .as_array()
            .unwrap()
            .len(),
        1
    );
    assert_eq!(s.query(&["--prefix", "x' OR 1=1 --"])["items"], json!([]));
}

#[test]
fn selectors_and_limits_are_rejected_without_opening_database() {
    let s = Sandbox::new();
    for args in [
        vec![],
        vec!["--uid", ""],
        vec!["--uid", " \t\n"],
        vec!["--uid", "x", "--prefix", "x"],
        vec!["--prefix", "", "--limit", "0"],
        vec!["--prefix", "", "--limit", "201"],
        vec!["--prefix", "", "--limit", "-1"],
        vec!["--uid", "x", "--limit", "50"],
        vec!["--prefix", "", "--scope", "x"],
        vec!["--prefix", "", "--cursor", "x"],
        vec!["--prefix", "", "--include-conflicts"],
    ] {
        let output = s.run(&args);
        assert!(!output.status.success(), "{args:?}");
        assert!(output.stdout.is_empty());
        assert!(
            !String::from_utf8_lossy(&output.stderr).contains("open existing"),
            "{args:?}: {}",
            String::from_utf8_lossy(&output.stderr)
        );
    }
    assert!(!s.path().exists());
}

#[test]
fn readonly_constructor_and_cli_do_not_create_missing_database_or_parents() {
    let s = Sandbox::new();
    let missing = s.root.join("absent/parents/index.db");
    assert!(NoteDb::open_read_only(&missing).is_err());
    assert!(!s.root.join("absent").exists());
    let out = s.run(&["--prefix", ""]);
    assert!(!out.status.success());
    assert!(out.stdout.is_empty());
    assert!(String::from_utf8_lossy(&out.stderr).contains("open existing notes index"));
    assert!(!s.path().exists());
}

#[test]
fn incompatible_legacy_schema_is_not_migrated() {
    let s = Sandbox::new();
    let db = s.db();
    db.connection()
        .execute_batch(
            "DROP TABLE note_ids; DROP TABLE note_id_conflicts;
         CREATE TABLE note_ids(note_id INTEGER, is_index INTEGER);
         CREATE TABLE note_id_conflicts(note_id INTEGER, is_index INTEGER);
         INSERT INTO note_ids VALUES (7,1);",
        )
        .unwrap();
    drop(db);
    let before = snapshot(&s.root);
    assert!(NoteDb::open_read_only(s.path()).is_err());
    assert_eq!(snapshot(&s.root), before);
    let out = s.run(&["--uid", "x"]);
    assert!(!out.status.success());
    assert!(out.stdout.is_empty());
    assert!(String::from_utf8_lossy(&out.stderr).contains("explicitly initialize/ingest/register"));
}

#[test]
fn conflicts_of_all_reasons_block_exact_and_completion_and_are_sorted_deduped() {
    let s = Sandbox::new();
    let db = s.db();
    insert(&db, "blocked", "registered.md", "registered");
    for (path, reason) in [
        ("z.md", "NOTE_NOT_IN_DB"),
        ("a.md", "DUPLICATE_ID"),
        ("a.md", "DUPLICATE_ID"),
        ("b.md", "MISSING_HUB_ID"),
    ] {
        conflict(&db, "blocked", path, reason);
    }
    conflict(&db, "only-conflicts", "c.md", "NOTE_NOT_IN_DB");
    drop(db);
    let v = s.query(&["--uid", "blocked"]);
    assert_eq!(v["status"], "conflict");
    assert!(v.get("target").is_none());
    let candidates = v["candidates"].as_array().unwrap();
    assert_eq!(candidates.len(), 4);
    assert_eq!(
        candidates[0],
        json!({"noteID":null,"path":"a.md","title":null,"isHub":false,"reason":"DUPLICATE_ID"})
    );
    assert_eq!(candidates[2]["reason"], "REGISTERED_WITH_CONFLICT");
    assert_eq!(s.query(&["--uid", "only-conflicts"])["status"], "conflict");
    assert_eq!(s.query(&["--prefix", ""])["items"], json!([]));
}

#[test]
fn full_conflict_group_has_no_prefix_limit() {
    let s = Sandbox::new();
    let db = s.db();
    for i in 0..205 {
        conflict(&db, "many", &format!("{i:03}.md"), "DUPLICATE_ID");
    }
    drop(db);
    assert_eq!(
        s.query(&["--uid", "many"])["candidates"]
            .as_array()
            .unwrap()
            .len(),
        205
    );
}

#[test]
fn dangling_and_path_mismatch_are_conflicts_not_targets_or_not_found() {
    let s = Sandbox::new();
    let db = s.db();
    insert(&db, "mismatch", "real.md", "title");
    db.connection()
        .execute_batch(
            "UPDATE note_ids SET path='wrong.md';
         PRAGMA foreign_keys=OFF;
         INSERT INTO note_ids(note_id,note_uid,path) VALUES (999,'dangling','gone.md');",
        )
        .unwrap();
    insert(&db, "valid", "valid.md", "");
    drop(db);
    for uid in ["mismatch", "dangling"] {
        let v = s.query(&["--uid", uid]);
        assert_eq!(v["status"], "conflict");
        assert_eq!(v["candidates"][0]["reason"], "INDEX_INCONSISTENT");
    }
    let v = s.query(&["--prefix", "", "--limit", "1"]);
    assert_eq!(v["items"][0]["uid"], "valid");
    assert_eq!(v["hasMore"], false);
}

#[test]
fn duplicate_owners_are_excluded_before_limit_without_arbitrary_winner() {
    let s = Sandbox::new();
    let db = s.db();
    db.connection().execute_batch(
        "DROP TABLE note_ids;
         CREATE TABLE note_ids(note_id INTEGER, note_uid TEXT COLLATE NOCASE, path TEXT, is_hub INTEGER);"
    ).unwrap();
    insert(&db, "aaa", "a.md", "one");
    insert(&db, "aaa", "b.md", "two");
    insert(&db, "FoO", "c.md", "");
    insert(&db, "foo", "d.md", "");
    drop(db);
    let v = s.query(&["--uid", "aaa"]);
    assert_eq!(v["status"], "conflict");
    assert_eq!(v["candidates"].as_array().unwrap().len(), 2);
    assert_eq!(v["candidates"][0]["reason"], "DUPLICATE_ID");
    let v = s.query(&["--prefix", "", "--limit", "1"]);
    assert_eq!(v["items"][0]["uid"], "FoO");
    assert_eq!(v["hasMore"], true);
    assert_eq!(s.query(&["--uid", "foo"])["target"]["path"], "d.md");
}

#[test]
fn not_found_has_no_target_or_candidates_and_readonly_connection_rejects_writes() {
    let s = Sandbox::new();
    drop(s.db());
    let db = NoteDb::open_read_only(s.path()).unwrap();
    assert!(db.connection().execute("DELETE FROM notes", []).is_err());
    drop(db);
    let v = s.query(&["--uid", "missing.md#fragment"]);
    assert_eq!(v["status"], "notFound");
    assert_eq!(v.as_object().unwrap().len(), 6);
    assert!(v.get("target").is_none());
    assert!(v.get("candidates").is_none());
}

#[test]
fn missing_registry_table_is_reported_without_recreating_it() {
    let s = Sandbox::new();
    let db = s.db();
    db.connection()
        .execute_batch("DROP TABLE note_id_conflicts;")
        .unwrap();
    drop(db);
    let out = s.run(&["--prefix", ""]);
    assert!(!out.status.success());
    assert!(out.stdout.is_empty());
    assert!(String::from_utf8_lossy(&out.stderr).contains("absent or incompatible"));
}

#[test]
fn invalid_candidate_is_an_actionable_error_not_an_invalid_dto() {
    let s = Sandbox::new();
    let db = s.db();
    conflict(&db, "bad", "", "DUPLICATE_ID");
    drop(db);
    let out = s.run(&["--uid", "bad"]);
    assert!(!out.status.success());
    assert!(out.stdout.is_empty());
    assert!(String::from_utf8_lossy(&out.stderr).contains("repair and explicitly rebuild"));
}

#[test]
fn config_overrides_work_without_executing_or_serializing_token_commands() {
    let s = Sandbox::new();
    insert(&s.db(), "id", "file.md", "");
    let marker = s.root.join("token-executed");
    fs::write(s.root.join("config/config.toml"), format!(
        "notes_dir = configured\ndb_path = index.db\n[hive_sync]\nenabled = true\ntoken_from_keychain = true\ntoken_command = {}\n",
        serde_json::to_string(&format!("/usr/bin/touch '{}'; printf secret", marker.display())).unwrap()
    )).unwrap();
    let before = snapshot(&s.root);
    let out = s
        .command(&["--uid", "id"])
        .env("NOTES_DIR", "~/other-notes")
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let v: Value = serde_json::from_slice(&out.stdout).unwrap();
    assert_eq!(
        v["notesDir"],
        s.root.join("home/other-notes").to_str().unwrap()
    );
    assert_eq!(snapshot(&s.root), before);
    assert!(!marker.exists());
    assert!(!v.to_string().contains("secret"));
}

#[test]
fn readonly_query_sees_committed_live_wal_data() {
    let s = Sandbox::new();
    let writer = s.db();
    writer
        .connection()
        .execute_batch("PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0;")
        .unwrap();
    insert(&writer, "wal-visible", "wal.md", "WAL title");
    assert!(s.path().with_extension("db-wal").exists());
    let before = snapshot(&s.root);
    let changes = writer.connection().total_changes();
    let out = s.command(&["--uid", "wal-visible"]).output().unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let v: Value = serde_json::from_slice(&out.stdout).unwrap();
    assert_eq!(v["status"], "resolved");
    assert_eq!(v["target"]["title"], "WAL title");
    assert_eq!(writer.connection().total_changes(), changes);
    // SQLite may update shared-memory reader/lock state, but not the DB or WAL data.
    for (path, bytes) in before {
        if path.extension().is_none_or(|ext| ext != "db-shm") {
            assert_eq!(fs::read(path).unwrap(), bytes);
        }
    }
}
