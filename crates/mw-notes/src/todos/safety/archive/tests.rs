use super::*;

struct Fixture {
    dir: tempfile::TempDir,
    root: PathBuf,
    note: PathBuf,
    dashboard: PathBuf,
    archive: PathBuf,
}
impl Fixture {
    fn new(tasks: &str) -> Self {
        let dir = tempfile::tempdir().unwrap();
        let root = fs::canonicalize(dir.path()).unwrap();
        let note = root.join("tasks.md");
        fs::write(&note, format!("---\nid: source\ndomains: [task]\ntask_active: true\ntask_scope: project\ntask_default_priority: p2\n---\n## Todo\n### Do Now\n{tasks}")).unwrap();
        Self {
            dashboard: root.join("dashboard.md"),
            archive: root.join("archive.md"),
            note,
            root,
            dir,
        }
    }
    fn todos(&self) -> Vec<TaskIndexTodo> {
        scan_task_snapshots(&self.root).unwrap().0
    }
    fn plan(&self, count: usize) -> ArchivePlan {
        let todos = self.todos();
        plan_archive(
            &self.root,
            &self.dashboard,
            &self.archive,
            &todos[..count]
                .iter()
                .map(|t| t.id.clone())
                .collect::<Vec<_>>(),
            &todos[0].source_revision,
        )
        .unwrap()
    }
    fn commit(&self, p: &ArchivePlan) -> io::Result<ArchiveResult> {
        commit_archive(&self.root, &self.dashboard, p)
    }
    fn inspect(&self, p: &ArchivePlan) -> ArchiveInspection {
        inspect_archive(&self.root, &self.dashboard, &self.archive, &p.operation_id).unwrap()
    }
    fn recover(&self, i: &ArchiveInspection) -> io::Result<ArchiveResult> {
        recover_archive(&self.root, &self.dashboard, i)
    }
}
fn hook(f: impl FnMut(&str) + 'static) {
    HOOK.with(|h| *h.borrow_mut() = Some(Box::new(f)));
}
fn clear() {
    HOOK.with(|h| *h.borrow_mut() = None);
}

#[test]
fn exact_payload_and_survivor_bytes_across_newlines_fences_and_markers() {
    for newline in ["\n", "\r\n"] {
        let block = "- [x] Finished area:Code\n\t- p1 e:s arbitrary prose\n\t<!-- mw-task:v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n\n\t```markdown\n# Todo\n- [ ] example\n<!-- mw-task:v1:BAD -->\n<!-- mw-archive-end:v1 -->\n\t```\n\tmore children\n".replace('\n', newline);
        let f = Fixture::new(&(block.clone() + "\n- [ ] Survivor\n  - area: Music\n- [ ] Last"));
        let original = fs::read(&f.note).unwrap();
        let p = f.plan(1);
        assert_eq!(p.raw_byte_count, block.len());
        assert_eq!(p.items[0].task.task_scope, "project");
        assert_eq!(p.items[0].task.metadata.default_priority, "p2");
        assert!(!f.root.join(".mw").exists());
        assert!(!f.archive.exists());
        assert!(!f.dashboard.exists());
        let r = f.commit(&p).unwrap();
        assert!(r.source_pruned && r.projection_updated);
        let mut expected = original[..p.items[0].byte_start].to_vec();
        expected.extend_from_slice(&original[p.items[0].byte_start + block.len()..]);
        assert_eq!(fs::read(&f.note).unwrap(), expected);
        let ledger = Ledger::read(&checked(&f.archive).unwrap()).unwrap();
        assert_eq!(ledger.records[0].payload, block.as_bytes());
        let survivors = f.todos();
        assert_eq!(survivors.len(), 2);
        assert_eq!(survivors[0].order, 0);
        assert_eq!(survivors[0].area, "Music");
        assert_eq!(survivors[0].source_revision, p.post_source_revision);
        let inspection = f.inspect(&p);
        assert_eq!(inspection.next_action, "none");
        let bytes = fs::read(&f.archive).unwrap();
        assert!(f.recover(&inspection).unwrap().already_complete);
        assert_eq!(fs::read(&f.archive).unwrap(), bytes);
    }
}

#[test]
fn identical_occurrences_are_not_deduplicated_and_separators_remain() {
    let f = Fixture::new("- [x] Same\n\n- [x] Same\n\n## Elsewhere\nUnrelated\n");
    let p = f.plan(2);
    assert_ne!(p.items[0].task.id, p.items[1].task.id);
    assert_eq!(p.raw_byte_count, 2 * "- [x] Same\n".len());
    f.commit(&p).unwrap();
    assert!(
        fs::read_to_string(&f.note)
            .unwrap()
            .ends_with("\n\n## Elsewhere\nUnrelated\n")
    );
    assert_eq!(
        Ledger::read(&checked(&f.archive).unwrap()).unwrap().records[0].payload,
        b"- [x] Same\n- [x] Same\n"
    );
}

#[test]
fn no_final_newline_and_unicode_payload() {
    let f = Fixture::new("- [x] \u{65e5}\u{672c}\u{8a9e}\n\tchild\r\n\tlast");
    let p = f.plan(1);
    let raw = fs::read(&f.note).unwrap()[p.items[0].byte_start..].to_vec();
    f.commit(&p).unwrap();
    assert_eq!(
        Ledger::read(&checked(&f.archive).unwrap()).unwrap().records[0].payload,
        raw
    );
}

#[test]
fn invalid_selections_and_unsupported_blocks_are_read_only() {
    for tasks in [
        "- [ ] Pending\n",
        "- [x] Done\nlazy prose\n",
        "- [x] Done\n  ```\nexample\n",
        "- [x] Done\n  - [ ] Nested\n",
        "- [x] Done\n  <!-- mw-task:v1:BAD -->\n",
    ] {
        let f = Fixture::new(tasks);
        let before = fs::read(&f.note).unwrap();
        let result = scan_task_snapshots(&f.root).and_then(|(ts, _, _)| {
            plan_archive(
                &f.root,
                &f.dashboard,
                &f.archive,
                &[ts[0].id.clone()],
                &ts[0].source_revision,
            )
        });
        assert!(result.is_err(), "{tasks}");
        assert_eq!(fs::read(&f.note).unwrap(), before);
        assert!(!f.root.join(".mw").exists());
        assert!(!f.archive.exists());
    }
    let f = Fixture::new("- [x] One\n- [x] Two\n");
    let ts = f.todos();
    for ids in [
        vec![],
        vec![ts[0].id.clone(), ts[0].id.clone()],
        vec!["missing".into()],
    ] {
        assert!(
            plan_archive(
                &f.root,
                &f.dashboard,
                &f.archive,
                &ids,
                &ts[0].source_revision
            )
            .is_err()
        );
    }
    assert!(
        plan_archive(
            &f.root,
            &f.dashboard,
            &f.archive,
            &[ts[0].id.clone()],
            "unchecked"
        )
        .is_err()
    );
    fs::write(
        f.root.join("second.md"),
        "---\nid: second\ndomains: [task]\ntask_active: true\n---\n## Todo\n### Do Now\n- [x] Other\n",
    )
    .unwrap();
    let ts = f.todos();
    assert!(
        plan_archive(
            &f.root,
            &f.dashboard,
            &f.archive,
            &ts.iter().map(|t| t.id.clone()).collect::<Vec<_>>(),
            &ts[0].source_revision
        )
        .is_err()
    );
}

#[test]
fn stale_archive_dashboard_source_and_plan_tampering_never_prune() {
    for edit in ["source", "dashboard", "archive", "plan", "created-archive"] {
        let f = Fixture::new("- [x] Done\n- [ ] Keep\n");
        let mut p = f.plan(1);
        match edit {
            "source" => fs::write(&f.note, fs::read_to_string(&f.note).unwrap() + "\n").unwrap(),
            "dashboard" => fs::write(&f.dashboard, "legacy").unwrap(),
            "archive" => fs::write(&f.archive, "human month notes").unwrap(),
            "created-archive" => {
                let other = f.plan(1);
                f.commit(&other).unwrap();
            }
            _ => p.items[0].task.text = "forged".into(),
        }
        let before = fs::read(&f.note).unwrap();
        let archive = fs::read(&f.archive).ok();
        let dashboard = fs::read(&f.dashboard).ok();
        assert!(f.commit(&p).is_err(), "{edit}");
        assert_eq!(fs::read(&f.note).unwrap(), before);
        assert_eq!(fs::read(&f.archive).ok(), archive);
        assert_eq!(fs::read(&f.dashboard).ok(), dashboard);
    }
}

#[test]
fn copy_first_faults_are_recoverable_without_duplicate_records_or_repruning() {
    for stage in [
        "archive-copy-published",
        "archive-pre-prune",
        "archive-source-pruned",
        "archive-before-projection",
        "archive-projection-published",
    ] {
        let f = Fixture::new("- [x] Done\n- [x] Same position now reused\n- [ ] Keep\n");
        let p = f.plan(1);
        let stage = stage.to_string();
        let fault_stage = stage.clone();
        let dashboard = f.dashboard.clone();
        hook(move |s| {
            if s == fault_stage {
                fs::write(&dashboard, "external dashboard edit").unwrap();
            }
        });
        let err = f.commit(&p).unwrap_err().to_string();
        clear();
        assert!(
            err.contains(&p.operation_id) && err.contains("durablePhase="),
            "{stage}: {err}"
        );
        let i = f.inspect(&p);
        assert!(i.blocked_reason.is_some());
        assert!(f.recover(&i).is_err());
        fs::remove_file(&f.dashboard).unwrap();
        let i = f.inspect(&p);
        assert!(
            i.blocked_reason.is_none(),
            "{stage}: {:?}",
            i.blocked_reason
        );
        let r = f.recover(&i).unwrap();
        assert_eq!(
            r.source_pruned,
            stage == "archive-copy-published" || stage == "archive-pre-prune"
        );
        assert_eq!(f.todos().len(), 2);
        assert_eq!(
            Ledger::read(&checked(&f.archive).unwrap())
                .unwrap()
                .records
                .len(),
            1
        );
        assert_eq!(f.inspect(&p).next_action, "none");
    }
}

#[test]
fn stale_inspection_binds_all_three_revisions() {
    for target in ["source", "archive", "dashboard"] {
        let f = Fixture::new("- [x] Done\n- [ ] Keep\n");
        let p = f.plan(1);
        f.commit(&p).unwrap();
        let i = f.inspect(&p);
        let path = match target {
            "source" => &f.note,
            "archive" => &f.archive,
            _ => &f.dashboard,
        };
        let mut bytes = fs::read(path).unwrap();
        bytes.push(b'\n');
        fs::write(path, &bytes).unwrap();
        let before = fs::read(&f.note).unwrap();
        assert!(f.recover(&i).is_err());
        assert_eq!(fs::read(&f.note).unwrap(), before);
        assert_eq!(fs::read(path).unwrap(), bytes);
    }
}

#[test]
fn restored_completed_source_is_blocked_and_clean_stale_projection_can_refresh() {
    let f = Fixture::new("- [x] Done\n- [ ] Keep\n");
    let pre = fs::read(&f.note).unwrap();
    let p = f.plan(1);
    f.commit(&p).unwrap();
    fs::remove_file(&f.dashboard).unwrap();
    let i = f.inspect(&p);
    assert_eq!(i.next_action, "refreshOnly");
    let r = f.recover(&i).unwrap();
    assert!(!r.source_pruned && r.projection_updated);
    fs::write(&f.note, &pre).unwrap();
    let i = f.inspect(&p);
    assert_eq!(i.next_action, "none");
    assert!(i.blocked_reason.is_some());
    assert!(f.recover(&i).is_err());
    assert_eq!(fs::read(&f.note).unwrap(), pre);
}

#[test]
fn corruption_and_external_archive_edits_preserve_evidence_before_prune() {
    for corrupt in [false, true] {
        let f = Fixture::new("- [x] Done\n- [ ] Keep\n");
        let p = f.plan(1);
        let path = f.archive.clone();
        hook(move |s| {
            if s == "archive-copy-published" {
                let mut bytes = fs::read(&path).unwrap();
                if corrupt {
                    let pos = bytes.windows(4).position(|w| w == b"Done").unwrap();
                    bytes[pos] = b'X';
                } else {
                    bytes.extend_from_slice(b"human edit");
                }
                fs::write(&path, bytes).unwrap();
            }
        });
        let pre = fs::read(&f.note).unwrap();
        assert!(f.commit(&p).is_err());
        clear();
        assert_eq!(fs::read(&f.note).unwrap(), pre);
        assert!(inspect_archive(&f.root, &f.dashboard, &f.archive, &p.operation_id).is_err());
    }
}

#[test]
fn locks_aliases_and_missing_parents_refuse() {
    let f = Fixture::new("- [x] Done\n");
    let p = f.plan(1);
    assert!(
        plan_archive(
            &f.root,
            &f.dashboard,
            f.root.join("missing/archive.md"),
            &[p.items[0].task.id.clone()],
            &p.source_revision
        )
        .is_err()
    );
    for archive in [&f.note, &f.dashboard] {
        assert!(
            plan_archive(
                &f.root,
                &f.dashboard,
                archive,
                &[p.items[0].task.id.clone()],
                &p.source_revision
            )
            .is_err()
        );
    }
    let root_lock = Lock::acquire(&f.root).unwrap();
    assert!(f.commit(&p).is_err());
    drop(root_lock);
    let locks = locks(&f.root, &f.archive).unwrap();
    assert!(f.commit(&p).is_err());
    let other = Fixture::new("- [x] Done\n");
    let op = plan_archive(
        &other.root,
        &other.dashboard,
        &f.archive,
        &[other.todos()[0].id.clone()],
        &other.todos()[0].source_revision,
    )
    .unwrap();
    assert!(other.commit(&op).is_err());
    drop(locks);
    assert!(!f.archive.exists());
    assert_eq!(f.todos().len(), 1);
    assert!(f.dir.path().exists());
}

#[cfg(unix)]
#[test]
fn symlinks_and_hardlinks_are_refused_for_every_role() {
    use std::os::unix::fs::symlink;
    for role in ["source", "dashboard", "archive"] {
        for hardlink in [false, true] {
            let f = Fixture::new("- [x] Done\n");
            let p = f.plan(1);
            let path = match role {
                "source" => &f.note,
                "dashboard" => &f.dashboard,
                _ => &f.archive,
            };
            let target = f.root.join("target.txt");
            if role == "source" {
                fs::rename(path, &target).unwrap();
            } else {
                fs::write(&target, "evidence").unwrap();
            }
            if hardlink {
                fs::hard_link(&target, path).unwrap();
            } else {
                symlink(&target, path).unwrap();
            }
            let before = fs::read(&target).unwrap();
            assert!(f.commit(&p).is_err());
            assert_eq!(fs::read(&target).unwrap(), before);
        }
    }
}

#[test]
fn strict_dtos_and_version_checks() {
    let f = Fixture::new("- [x] Done\n");
    let p = f.plan(1);
    for field in ["root", "task", "metadata"] {
        let mut v = serde_json::to_value(&p).unwrap();
        let obj = match field {
            "root" => &mut v,
            "task" => &mut v["items"][0]["task"],
            _ => &mut v["items"][0]["task"]["metadata"],
        };
        obj["unexpected"] = true.into();
        assert!(serde_json::from_value::<ArchivePlan>(v).is_err());
    }
    let mut v = p.clone();
    v.schema_version = "2".into();
    assert!(f.commit(&v).is_err());
    let json = serde_json::to_string(&p).unwrap();
    for json in [
        json.replacen("\"id\":", "\"id\":\"duplicate\",\"id\":", 1),
        json.replacen("\"status\":", "\"status\":\"duplicate\",\"status\":", 1),
    ] {
        assert!(serde_json::from_str::<ArchivePlan>(&json).is_err());
    }
    f.commit(&p).unwrap();
    let mut inspection = serde_json::to_value(f.inspect(&p)).unwrap();
    inspection.as_object_mut().unwrap().remove("blockedReason");
    assert!(serde_json::from_value::<ArchiveInspection>(inspection).is_err());
    let mut v = p;
    v.items[0].byte_start = usize::MAX;
    assert!(f.commit(&v).is_err());
}

#[test]
fn append_preserves_previous_bytes_and_distinct_roots_can_share_a_ledger() {
    let first = Fixture::new("- [x] Same\n- [ ] Keep\n");
    let p = first.plan(1);
    first.commit(&p).unwrap();
    let old = fs::read(&first.archive).unwrap();
    let second = Fixture::new("- [x] Same\n- [ ] Other\n");
    let todo = second.todos().remove(0);
    let p2 = plan_archive(
        &second.root,
        &second.dashboard,
        &first.archive,
        &[todo.id],
        &todo.source_revision,
    )
    .unwrap();
    second.commit(&p2).unwrap();
    let bytes = fs::read(&first.archive).unwrap();
    assert!(bytes.starts_with(&old));
    let ledger = Ledger::read(&checked(&first.archive).unwrap()).unwrap();
    assert_eq!(ledger.records.len(), 2);
    assert_ne!(p.operation_id, p2.operation_id);
    assert!(
        inspect_archive(
            &first.root,
            &first.dashboard,
            &first.archive,
            &p2.operation_id
        )
        .is_err()
    );
    assert_eq!(
        inspect_archive(
            &second.root,
            &second.dashboard,
            &first.archive,
            &p2.operation_id
        )
        .unwrap()
        .next_action,
        "none"
    );
}

#[test]
fn source_races_before_and_after_prune_are_not_overwritten_or_recovered_blindly() {
    for stage in [
        "archive-before-copy",
        "archive-copy-published",
        "archive-pre-prune",
        "archive-source-pruned",
        "archive-before-projection",
        "archive-projection-published",
        "archive-before-phase-update",
    ] {
        let f = Fixture::new("- [x] Done\n- [ ] Keep\n");
        let p = f.plan(1);
        let note = f.note.clone();
        let stage = stage.to_string();
        let fault = stage.clone();
        hook(move |s| {
            if s == fault {
                fs::write(&note, "external source edit").unwrap();
            }
        });
        assert!(f.commit(&p).is_err(), "{stage}");
        clear();
        assert_eq!(fs::read_to_string(&f.note).unwrap(), "external source edit");
        if f.archive.exists() {
            let i = f.inspect(&p);
            assert!(i.blocked_reason.is_some());
            assert!(f.recover(&i).is_err());
        }
    }
}

#[test]
fn recovery_rechecks_observations_after_inspection_before_any_mutation() {
    for target in ["source", "archive", "dashboard"] {
        let f = Fixture::new("- [x] Done\n- [ ] Keep\n");
        let p = f.plan(1);
        f.commit(&p).unwrap();
        fs::remove_file(&f.dashboard).unwrap();
        let i = f.inspect(&p);
        let path = match target {
            "source" => f.note.clone(),
            "archive" => f.archive.clone(),
            _ => f.dashboard.clone(),
        };
        let changed = path.clone();
        hook(move |s| {
            if s == "archive-recovery-after-inspect" {
                fs::write(&changed, "external edit").unwrap();
            }
        });
        assert!(f.recover(&i).is_err());
        clear();
        assert_eq!(fs::read_to_string(&path).unwrap(), "external edit");
    }
}

#[test]
fn pending_legacy_and_layout_edits_block_plan_and_recovery() {
    for edit in ["pending", "legacy", "layout"] {
        let f = Fixture::new("- [x] Done\n- [ ] Keep\n");
        project(&f.root, &f.dashboard, false, false).unwrap();
        let p = f.plan(1);
        let clean_dashboard = fs::read(&f.dashboard).unwrap();
        let dirty = match edit {
            "pending" => String::from_utf8(clean_dashboard.clone())
                .unwrap()
                .replace("- [ ] Keep", "- [x] Keep"),
            "legacy" => "legacy dashboard".into(),
            _ => String::from_utf8(clean_dashboard.clone())
                .unwrap()
                .replace("## Action", "## Wrong"),
        };
        fs::write(&f.dashboard, &dirty).unwrap();
        let t = f.todos().remove(0);
        assert!(
            plan_archive(
                &f.root,
                &f.dashboard,
                &f.archive,
                &[t.id],
                &t.source_revision
            )
            .is_err()
        );
        assert!(!f.archive.exists());
        fs::write(&f.dashboard, clean_dashboard).unwrap();
        f.commit(&p).unwrap();
        fs::write(&f.dashboard, &dirty).unwrap();
        let i = f.inspect(&p);
        assert!(i.blocked_reason.is_some());
        assert!(f.recover(&i).is_err());
        assert_eq!(fs::read_to_string(&f.dashboard).unwrap(), dirty);
    }
}

#[test]
fn inode_mode_and_surviving_stable_identity_are_preserved() {
    let f = Fixture::new(
        "- [x] Done\n- [ ] Stable\n  <!-- mw-task:v1:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb -->\n",
    );
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(&f.note, fs::Permissions::from_mode(0o640)).unwrap();
    }
    let p = f.plan(1);
    f.commit(&p).unwrap();
    assert_eq!(f.todos()[0].id, "task:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb");
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        assert_eq!(
            fs::metadata(&f.note).unwrap().permissions().mode() & 0o777,
            0o640
        );
    }
}

#[test]
fn bounds_empty_ledgers_and_corrupt_receipts_fail_closed_without_panics() {
    let f = Fixture::new("- [x] Done\n");
    let p = f.plan(1);
    f.commit(&p).unwrap();
    let original = fs::read(&f.archive).unwrap();
    for bytes in [Vec::new(), b"---\nid: mw-archive-\xe6\x97\xa5\xe6\x97\xa5\xe6\x97\xa5\xe6\x97\xa5\xe6\x97\xa5\xe6\x97\xa5\xe6\x97\xa5\xe6\x97\xa5\xe6\x97\xa5\xe6\x97\xa5\xe6\x97\xa5\ntask_active: false\n---\n<!-- mw-archive-ledger:v1 -->\n".to_vec(), original[..original.len()-1].to_vec()] {
        fs::write(&f.archive, bytes).unwrap(); assert!(inspect_archive(&f.root, &f.dashboard, &f.archive, &p.operation_id).is_err());
    }
    fs::write(&f.archive, &original).unwrap();
    let mut ledger = Ledger::read(&checked(&f.archive).unwrap()).unwrap();
    ledger.records[0].receipt.plan.items[0].byte_length = usize::MAX;
    fs::write(&f.archive, ledger.bytes().unwrap()).unwrap();
    assert!(inspect_archive(&f.root, &f.dashboard, &f.archive, &p.operation_id).is_err());
    let too_many = Fixture::new(&"- [x] Done\n".repeat(MAX_TASKS + 1));
    let todos = too_many.todos();
    assert!(
        plan_archive(
            &too_many.root,
            &too_many.dashboard,
            &too_many.archive,
            &todos.iter().map(|t| t.id.clone()).collect::<Vec<_>>(),
            &todos[0].source_revision
        )
        .is_err()
    );
}

#[test]
fn managed_archive_created_or_changed_and_clean_dashboard_revision_changes_are_stale() {
    for edit in ["created", "changed", "dashboard"] {
        let f = Fixture::new("- [x] Done\n- [ ] Keep\n");
        if edit == "dashboard" {
            project(&f.root, &f.dashboard, false, false).unwrap();
        }
        let other = Fixture::new("- [x] Other\n- [x] Another\n");
        let append = || {
            let t = other.todos().remove(0);
            let plan = plan_archive(
                &other.root,
                &other.dashboard,
                &f.archive,
                &[t.id],
                &t.source_revision,
            )
            .unwrap();
            other.commit(&plan).unwrap();
        };
        if edit == "changed" {
            append();
        }
        let p = f.plan(1);
        if edit == "dashboard" {
            let mut bytes = fs::read(&f.dashboard).unwrap();
            bytes.push(b'\n');
            fs::write(&f.dashboard, bytes).unwrap();
        } else {
            append();
        }
        let source = fs::read(&f.note).unwrap();
        let archive = fs::read(&f.archive).ok();
        let dashboard = fs::read(&f.dashboard).ok();
        assert!(f.commit(&p).is_err(), "{edit}");
        assert_eq!(fs::read(&f.note).unwrap(), source);
        assert_eq!(fs::read(&f.archive).ok(), archive);
        assert_eq!(fs::read(&f.dashboard).ok(), dashboard);
    }
}

#[cfg(unix)]
#[test]
fn literal_parent_symlink_dotdot_is_resolved_by_filesystem_not_lexically() {
    use std::os::unix::fs::symlink;
    let f = Fixture::new("- [x] Done\n");
    fs::create_dir(f.root.join("actual")).unwrap();
    fs::create_dir(f.root.join("actual/child")).unwrap();
    symlink(f.root.join("actual/child"), f.root.join("alias")).unwrap();
    let t = f.todos().remove(0);
    let p = plan_archive(
        &f.root,
        &f.dashboard,
        f.root.join("alias/../ledger.md"),
        &[t.id],
        &t.source_revision,
    )
    .unwrap();
    assert_eq!(
        p.archive_path,
        utf8(&f.root.join("actual/ledger.md")).unwrap()
    );
    assert!(!f.root.join(".mw").exists());
}

#[test]
fn unfenced_indented_parser_headings_refuse_without_any_writes() {
    for tail in [
        "  ## Blocked\n  - [ ] Must remain\n- [ ] Other survivor\n",
        "  ## Blocked\n- [ ] Survivor with Blocked context\n",
        "  * prose\n- [ ] Survivor with prose context\n",
    ] {
        let f = Fixture::new(&format!("- [x] Selected\n{tail}"));
        let todos = f.todos();
        let before = fs::read(&f.note).unwrap();
        let error = plan_archive(
            &f.root,
            &f.dashboard,
            &f.archive,
            &[todos[0].id.clone()],
            &todos[0].source_revision,
        )
        .unwrap_err();
        assert!(error.to_string().contains("unfenced indented heading"));
        assert_eq!(fs::read(&f.note).unwrap(), before);
        assert!(!f.root.join(".mw").exists());
        assert!(!f.dashboard.exists());
        assert!(!f.archive.exists());
        assert_eq!(f.todos(), todos);
    }
}

#[test]
fn owned_fenced_headings_and_checkboxes_remain_exact_payload() {
    let block =
        "- [x] Selected\n  ```markdown\n  ## Waiting\n  - [ ] Example only\n  * prose\n  ```\n";
    let f = Fixture::new(&format!(
        "{block}- [ ] Must remain\n  <!-- mw-task:v1:cccccccccccccccccccccccccccccccc -->\n"
    ));
    let original = f.todos();
    assert_eq!(original.len(), 2);
    let p = f.plan(1);
    f.commit(&p).unwrap();
    assert_eq!(
        Ledger::read(&checked(&f.archive).unwrap()).unwrap().records[0].payload,
        block.as_bytes()
    );
    let survivor = f.todos().remove(0);
    assert_eq!(survivor.id, original[1].id);
    assert_eq!(survivor.todo_section, "Do Now");
    assert_eq!(survivor.text, "Must remain");
}

#[test]
fn survivor_proof_rejects_unselected_removal_and_same_count_context_changes() {
    let f = Fixture::new("- [x] Selected\n- [ ] Must remain\n- [ ] Other survivor\n");
    let p = f.plan(1);
    let todos = f.todos();
    let original = fs::read_to_string(&f.note).unwrap();
    let mut items = p.items.clone();
    let start = items[0].byte_start;
    items[0].byte_length += "- [ ] Must remain\n".len();
    let post = format!(
        "{}{}",
        &original[..start],
        &original[start + items[0].byte_length..]
    );
    let survivors = scan_task_snapshots_with(&f.root, Some(("tasks.md", &post)))
        .unwrap()
        .0;
    assert!(
        prove_survivors(
            &todos,
            &survivors,
            "tasks.md",
            &original,
            post.as_bytes(),
            &items
        )
        .unwrap_err()
        .to_string()
        .contains("unselected task identity")
    );

    let post = format!(
        "{}{}",
        &original[..start],
        &original[start + p.items[0].byte_length..]
    );
    let mut survivors = scan_task_snapshots_with(&f.root, Some(("tasks.md", &post)))
        .unwrap()
        .0;
    survivors[0].todo_section = "Blocked".into();
    assert!(
        prove_survivors(
            &todos,
            &survivors,
            "tasks.md",
            &original,
            post.as_bytes(),
            &p.items
        )
        .is_err()
    );
    // Coordinates alone cannot substitute another occurrence's identity.
    survivors = scan_task_snapshots_with(&f.root, Some(("tasks.md", &post)))
        .unwrap()
        .0;
    survivors[0].id = todos[2].id.clone();
    assert!(
        prove_survivors(
            &todos,
            &survivors,
            "tasks.md",
            &original,
            post.as_bytes(),
            &p.items
        )
        .is_err()
    );
}

#[test]
fn due_start_clock_snapshot_does_not_block_delayed_commit_or_recovery() {
    for recover in [false, true] {
        let f = Fixture::new("- [x] Selected due:2030-01-01 start:2030-01-09\n- [ ] Keep\n");
        let mut confirmed = f.plan(1);
        // Model Jan 1 -> Jan 2: due 0 -> -1 days, start 8 -> 7 days, without the wall clock.
        confirmed.items[0].task.metadata.effective_weight = 1.5 * 1.25 * 0.70;
        let mut next_day = confirmed.clone();
        next_day.items[0].task.metadata.effective_weight = 1.5 * 1.35 * 0.85;
        assert_ne!(confirmed, next_day);
        assert!(same_plan_semantics(&confirmed, &next_day));
        let mut fresh = f.plan(1);
        fresh.operation_id = confirmed.operation_id.clone();
        assert!(same_plan_semantics(&fresh, &confirmed));
        let original = fs::read(&f.note).unwrap();
        if recover {
            let dashboard = f.dashboard.clone();
            hook(move |s| {
                if s == "archive-copy-published" {
                    fs::write(&dashboard, "pending external edit").unwrap();
                }
            });
            assert!(f.commit(&confirmed).is_err());
            clear();
            assert_eq!(fs::read(&f.note).unwrap(), original);
            fs::remove_file(&f.dashboard).unwrap();
            let inspection = f.inspect(&confirmed);
            assert_eq!(inspection.plan, confirmed);
            assert_eq!(inspection.next_action, "resumePrune");
            assert!(f.recover(&inspection).unwrap().source_pruned);
        } else {
            f.commit(&confirmed).unwrap();
        }
        let ledger = Ledger::read(&checked(&f.archive).unwrap()).unwrap();
        assert_eq!(ledger.records[0].receipt.plan, confirmed);
        let item = &confirmed.items[0];
        assert_eq!(
            ledger.records[0].payload,
            original[item.byte_start..item.byte_start + item.byte_length]
        );
        assert_eq!(f.inspect(&confirmed).plan, confirmed);
    }
}

#[test]
fn clock_normalization_does_not_ignore_source_configuration_or_receipt_edits() {
    for field in ["due", "start", "priority", "weight", "raw", "default"] {
        let f = Fixture::new("- [x] Selected due:2030-01-01 start:2030-01-09\n- [ ] Keep\n");
        let mut p = f.plan(1);
        match field {
            "due" => p.items[0].task.metadata.due = "2030-01-02".into(),
            "start" => p.items[0].task.metadata.start = "2030-01-10".into(),
            "priority" => p.items[0].task.metadata.priority = "p1".into(),
            "weight" => p.items[0].task.metadata.weight_override = "42".into(),
            "raw" => p.items[0].task.metadata.raw = "changed".into(),
            _ => p.items[0].task.metadata.default_priority = "p1".into(),
        }
        let before = fs::read(&f.note).unwrap();
        assert!(f.commit(&p).is_err());
        assert_eq!(fs::read(&f.note).unwrap(), before);
        assert!(!f.archive.exists());
    }
    for receipt in [false, true] {
        let f = Fixture::new("- [x] Selected due:2030-01-01 start:2030-01-09\n- [ ] Keep\n");
        let p = f.plan(1);
        let dashboard = f.dashboard.clone();
        hook(move |s| {
            if s == "archive-copy-published" {
                fs::write(&dashboard, "external edit").unwrap();
            }
        });
        assert!(f.commit(&p).is_err());
        clear();
        fs::remove_file(&f.dashboard).unwrap();
        if receipt {
            let mut ledger = Ledger::read(&checked(&f.archive).unwrap()).unwrap();
            ledger.records[0].receipt.plan.items[0].task.metadata.due = "2030-01-02".into();
            fs::write(&f.archive, ledger.bytes().unwrap()).unwrap();
        } else {
            fs::write(
                &f.note,
                fs::read_to_string(&f.note)
                    .unwrap()
                    .replace("2030-01-01", "2030-01-02"),
            )
            .unwrap();
        }
        let before = fs::read(&f.note).unwrap();
        let inspection = f.inspect(&p);
        assert!(inspection.blocked_reason.is_some());
        assert!(f.recover(&inspection).is_err());
        assert_eq!(fs::read(&f.note).unwrap(), before);
    }
}

#[test]
fn second_record_phase_failure_recovery_preserves_first_record_and_post_state() {
    let f = Fixture::new("- [x] First\n- [x] Second\n- [ ] Keep\n");
    let first = f.plan(1);
    f.commit(&first).unwrap();
    let first_bytes = fs::read(&f.archive).unwrap();
    let second = f.plan(1);
    let archive = f.archive.clone();
    let copy = std::rc::Rc::new(std::cell::RefCell::new(None));
    let captured = copy.clone();
    hook(move |s| {
        if s == "archive-before-phase-update" {
            let bytes = fs::read(&archive).unwrap();
            *captured.borrow_mut() = Some(bytes.clone());
            let mut changed = bytes;
            changed.extend_from_slice(b"external edit");
            fs::write(&archive, changed).unwrap();
        }
    });
    let err = f.commit(&second).unwrap_err().to_string();
    clear();
    assert!(err.contains("durablePhase=archiveCopied"));
    let post = fs::read(&f.note).unwrap();
    assert_eq!(revision(&post), second.post_source_revision);
    assert_eq!(f.todos()[0].id, second.items[0].task.id);
    // Restore only the captured verified copy in this fixture, then explicitly inspect/recover.
    fs::write(&f.archive, copy.borrow().as_ref().unwrap()).unwrap();
    let ledger = Ledger::read(&checked(&f.archive).unwrap()).unwrap();
    let second_payload = ledger.records[1].payload.clone();
    let inspection = f.inspect(&second);
    assert_eq!(inspection.source_state, "post");
    assert_eq!(inspection.next_action, "refreshOnly");
    assert!(!f.recover(&inspection).unwrap().source_pruned);
    assert_eq!(fs::read(&f.note).unwrap(), post);
    assert!(fs::read(&f.archive).unwrap().starts_with(&first_bytes));
    let ledger = Ledger::read(&checked(&f.archive).unwrap()).unwrap();
    assert_eq!(ledger.records.len(), 2);
    assert_eq!(ledger.records[1].payload, second_payload);
    assert_eq!(ledger.records[1].receipt.plan, second);
    assert_eq!(f.todos()[0].text, "Keep");
}

#[cfg(unix)]
#[test]
fn valid_managed_ledger_hardlinks_are_refused_by_the_link_gate() {
    let f = Fixture::new("- [x] First\n- [x] Second\n");
    let p = f.plan(1);
    f.commit(&p).unwrap();
    let next = f.plan(1);
    let alias = f.root.join("alias.md");
    fs::hard_link(&f.archive, &alias).unwrap();
    let bytes = fs::read(&f.archive).unwrap();
    // The bytes are valid managed grammar; rejection must come from the hardlink gate.
    assert!(Ledger::read(&Snapshot::read(&f.archive).unwrap()).is_ok());
    let err = f.commit(&next).unwrap_err().to_string();
    assert!(err.contains("hardlinked file refused"));
    let err = inspect_archive(&f.root, &f.dashboard, &f.archive, &p.operation_id)
        .unwrap_err()
        .to_string();
    assert!(err.contains("hardlinked file refused"));
    assert_eq!(fs::read(&f.archive).unwrap(), bytes);
    assert_eq!(fs::read(&alias).unwrap(), bytes);
    assert_eq!(f.todos().len(), 1);
}

#[test]
fn both_lock_orders_release_the_first_lock_when_the_second_is_busy() {
    let f = Fixture::new("- [x] Selected\n");
    for root_first in [true, false] {
        let root = f.root.join(if root_first { "a-root" } else { "z-root" });
        let parent = f.root.join(if root_first {
            "z-destination"
        } else {
            "a-destination"
        });
        fs::create_dir(&root).unwrap();
        fs::create_dir(&parent).unwrap();
        let archive = parent.join("ledger.md");
        let destination_lock = parent.join(".ledger.md.mw-archive.lock");
        let root_lock = root.join(".mw/todos.lock");
        assert_eq!(root_lock < destination_lock, root_first);
        if root_first {
            fs::write(&destination_lock, "occupied destination lock").unwrap();
            assert!(locks(&root, &archive).is_err());
            assert!(!root_lock.exists());
            assert_eq!(
                fs::read_to_string(&destination_lock).unwrap(),
                "occupied destination lock"
            );
            fs::remove_file(&destination_lock).unwrap();
        } else {
            let occupied = Lock::acquire(&root).unwrap();
            assert!(locks(&root, &archive).is_err());
            assert!(!destination_lock.exists());
            assert!(root_lock.exists());
            drop(occupied);
        }
        let guards = locks(&root, &archive).unwrap();
        assert!(root_lock.exists() && destination_lock.exists());
        drop(guards);
        assert!(!root_lock.exists() && !destination_lock.exists());
        assert!(!archive.exists());
    }
}

#[test]
fn frozen_v1_historical_bytes_are_canonical_and_recover_without_live_wire_schema() {
    // Hand-checked V1 fixture: one 37-byte block at offset 120, positional source:10:0.
    let bytes = include_bytes!("v1-ledger.md");
    let snapshot = Snapshot {
        path: PathBuf::from("/archive-v1-fixture/archive.md"),
        bytes: Some(bytes.to_vec()),
    };
    let mut ledger = Ledger::read(&snapshot).unwrap();
    assert_eq!(ledger.bytes().unwrap(), bytes);
    let historical_task = ledger.records[0].receipt.plan.items[0].task.clone();
    assert_eq!(
        historical_task.metadata.effective_weight.to_bits(),
        51.248178375505404_f64.to_bits()
    );
    assert_eq!(
        historical_task.metadata.weight_override,
        "51.248178375505404"
    );
    assert!(historical_task.stable_id.is_none());
    for phase in ["sourcePruned", "completed", "archiveCopied"] {
        ledger.records[0].receipt.phase = phase.into();
        let updated = Snapshot {
            path: snapshot.path.clone(),
            bytes: Some(ledger.bytes().unwrap()),
        };
        let parsed = Ledger::read(&updated).unwrap();
        assert_eq!(
            parsed.records[0].receipt.plan.items[0].task,
            historical_task
        );
        assert_eq!(parsed.bytes().unwrap(), updated.bytes.unwrap());
    }
    assert_eq!(ledger.bytes().unwrap(), bytes);

    let f = Fixture::new("");
    // Recovery must use the exact historical source bound by the frozen receipt.
    fs::write(&f.note, "---\nid: source\ndomains: [task-index]\ntask_active: true\ntask_scope: project\ntask_default_priority: p2\n---\n# Todo\n## Next\n- [x] Precision w:51.248178375505404\n- [ ] Keep\n").unwrap();
    // Only relocate the frozen fixture's three filesystem identities; task JSON is never rebuilt.
    let p = &mut ledger.records[0].receipt.plan;
    p.notes_dir = utf8(&f.root).unwrap();
    p.dashboard_path = utf8(&f.dashboard).unwrap();
    p.archive_path = utf8(&f.archive).unwrap();
    let confirmed = p.clone();
    assert_eq!(revision(&fs::read(&f.note).unwrap()), p.source_revision);
    fs::write(&f.archive, ledger.bytes().unwrap()).unwrap();
    let inspection = f.inspect(&confirmed);
    assert_eq!(
        inspection.next_action, "resumePrune",
        "{:?}",
        inspection.blocked_reason
    );
    assert_eq!(inspection.plan.items[0].task, historical_task);
    assert!(f.recover(&inspection).unwrap().source_pruned);
    let completed = Ledger::read(&checked(&f.archive).unwrap()).unwrap();
    assert_eq!(completed.records[0].receipt.plan, confirmed);
    assert_eq!(completed.records[0].payload, ledger.records[0].payload);
    assert_eq!(completed.records[0].receipt.phase, "completed");
    assert_eq!(completed.bytes().unwrap(), fs::read(&f.archive).unwrap());
}

#[test]
fn frozen_v1_context_has_exact_keys_required_null_and_explicit_live_conversion() {
    const TASK_FIELDS: &[&str] = &[
        "id",
        "stableId",
        "identityKind",
        "sourceRevision",
        "sourceId",
        "sourcePath",
        "noteTitle",
        "taskScope",
        "todoSection",
        "area",
        "text",
        "done",
        "order",
        "line",
        "metadata",
    ];
    const META_FIELDS: &[&str] = &[
        "status",
        "todoSection",
        "area",
        "priority",
        "energy",
        "weightOverride",
        "due",
        "start",
        "estimate",
        "raw",
        "effectiveWeight",
        "defaultPriority",
        "defaultEnergy",
    ];
    let snapshot = Snapshot {
        path: PathBuf::from("/archive-v1-fixture/archive.md"),
        bytes: Some(include_bytes!("v1-ledger.md").to_vec()),
    };
    let ledger = Ledger::read(&snapshot).unwrap();
    let task = ledger.records[0].receipt.plan.items[0].task.clone();
    let value = serde_json::to_value(&task).unwrap();
    assert_eq!(value.as_object().unwrap().len(), TASK_FIELDS.len());
    assert_eq!(
        value["metadata"].as_object().unwrap().len(),
        META_FIELDS.len()
    );
    for (fields, metadata) in [(TASK_FIELDS, false), (META_FIELDS, true)] {
        for field in fields {
            let mut missing = value.clone();
            let object = if metadata {
                &mut missing["metadata"]
            } else {
                &mut missing
            };
            object.as_object_mut().unwrap().remove(*field);
            assert!(
                serde_json::from_value::<ArchiveTaskContextV1>(missing).is_err(),
                "missing {field}"
            );
        }
        let mut extra = value.clone();
        let object = if metadata {
            &mut extra["metadata"]
        } else {
            &mut extra
        };
        object["futureLiveField"] = true.into();
        assert!(serde_json::from_value::<ArchiveTaskContextV1>(extra).is_err());
    }
    let json = serde_json::to_string(&task).unwrap();
    for duplicate in [
        json.replacen(
            "\"stableId\":null",
            "\"stableId\":null,\"stableId\":null",
            1,
        ),
        json.replacen(
            "\"effectiveWeight\":",
            "\"effectiveWeight\":1.0,\"effectiveWeight\":",
            1,
        ),
    ] {
        assert!(serde_json::from_str::<ArchiveTaskContextV1>(&duplicate).is_err());
    }
    let f = Fixture::new("- [x] Precision w:51.248178375505404\n- [ ] Keep\n");
    let live = f.todos().remove(0);
    let converted: ArchiveTaskContextV1 = live.clone().into();
    assert_eq!(task.todo_section, "Next");
    assert_eq!(task.metadata.todo_section, "Next");
    assert_eq!(converted.todo_section, "Do Now");
    assert_eq!(converted.metadata.todo_section, "Do Now");
    assert_ne!(converted.source_revision, task.source_revision);
    // Compare the other fields without rewriting the historical wire context.
    let mut expected = task;
    expected.todo_section = converted.todo_section.clone();
    expected.metadata.todo_section = converted.metadata.todo_section.clone();
    expected.source_revision = converted.source_revision.clone();
    assert_eq!(converted, expected);
    assert_eq!(
        ArchiveTodoMetadataV1::from(live.metadata),
        converted.metadata
    );
}

#[test]
fn public_archive_protocol_preserves_precision_float_bits_in_copy_and_completed_receipts() {
    let expected = 51.248178375505404_f64;
    for copied_pre_recovery in [false, true] {
        let f = Fixture::new("- [x] Precision w:51.248178375505404\n- [ ] Keep\n");
        let live = crate::list_active_task_index_todos(&f.root)
            .unwrap()
            .0
            .remove(0);
        let plan = crate::plan_archive(
            &f.root,
            &f.dashboard,
            &f.archive,
            &[live.id],
            &live.source_revision,
        )
        .unwrap();
        assert_eq!(
            plan.items[0].task.metadata.effective_weight.to_bits(),
            expected.to_bits()
        );
        let plan_bytes = serde_json::to_vec(&plan).unwrap();
        assert!(
            std::str::from_utf8(&plan_bytes)
                .unwrap()
                .contains("\"effectiveWeight\":51.248178375505404")
        );
        let confirmed: crate::ArchivePlan = serde_json::from_slice(&plan_bytes).unwrap();
        assert_eq!(
            confirmed.items[0].task.metadata.effective_weight.to_bits(),
            expected.to_bits()
        );
        if copied_pre_recovery {
            let dashboard = f.dashboard.clone();
            hook(move |stage| {
                if stage == "archive-copy-published" {
                    fs::write(&dashboard, "external edit").unwrap();
                }
            });
            let result = crate::commit_archive(&f.root, &f.dashboard, &confirmed);
            clear();
            assert!(result.is_err());
            let copy = Ledger::read(&checked(&f.archive).unwrap()).unwrap();
            assert_eq!(copy.records[0].receipt.phase, "archiveCopied");
            assert_eq!(copy.records[0].receipt.plan, plan);
            assert_eq!(
                copy.records[0].receipt.plan.items[0]
                    .task
                    .metadata
                    .effective_weight
                    .to_bits(),
                expected.to_bits()
            );
            assert_eq!(copy.bytes().unwrap(), fs::read(&f.archive).unwrap());
            fs::remove_file(&f.dashboard).unwrap();
        } else {
            crate::commit_archive(&f.root, &f.dashboard, &confirmed).unwrap();
        }
        let inspected =
            crate::inspect_archive(&f.root, &f.dashboard, &f.archive, &plan.operation_id).unwrap();
        assert_eq!(inspected.plan, plan);
        let observed: crate::ArchiveInspection =
            serde_json::from_slice(&serde_json::to_vec(&inspected).unwrap()).unwrap();
        assert_eq!(
            observed.plan.items[0]
                .task
                .metadata
                .effective_weight
                .to_bits(),
            expected.to_bits()
        );
        crate::recover_archive(&f.root, &f.dashboard, &observed).unwrap();
        let completed = Ledger::read(&checked(&f.archive).unwrap()).unwrap();
        assert_eq!(completed.records[0].receipt.phase, "completed");
        assert_eq!(completed.records[0].receipt.plan, plan);
        assert_eq!(
            completed.records[0].receipt.plan.items[0]
                .task
                .metadata
                .effective_weight
                .to_bits(),
            expected.to_bits()
        );
        assert_eq!(completed.bytes().unwrap(), fs::read(&f.archive).unwrap());
    }
}

#[test]
fn library_format_preserves_completed_ledgers_in_regular_and_hub_destinations() {
    for rel in ["archive.md", "hub.md", "area/hub.md"] {
        let mut f = Fixture::new(
            "- [x] Selected\r\n  ```markdown\r\n# Example raw heading\n- [ ] Example only\n  ```\r\n  child\tbytes\r\n- [x] Next archive\n- [ ] Keep\n",
        );
        if rel.starts_with("area/") {
            fs::create_dir(f.root.join("area")).unwrap();
        }
        f.archive = f.root.join(rel);
        // A regular source without H1 is intentionally changed by generic formatting.
        let plan = f.plan(1);
        f.commit(&plan).unwrap();
        let original = fs::read(&f.archive).unwrap();
        assert_eq!(
            crate::normalize_markdown_headings(std::str::from_utf8(&original).unwrap(), "unused")
                .as_bytes(),
            original
        );
        crate::format_notes(&f.root, true).unwrap();
        crate::format_notes(&f.root, false).unwrap();
        assert_eq!(fs::read(&f.archive).unwrap(), original, "{rel}");
        assert_eq!(
            Ledger::read(&checked(&f.archive).unwrap())
                .unwrap()
                .bytes()
                .unwrap(),
            original
        );
        let inspection = f.inspect(&plan);
        assert_eq!(inspection.phase, "completed");
        assert_eq!(inspection.source_state, "changed");
        assert!(inspection.blocked_reason.is_some());
        let next = f.plan(1);
        assert_ne!(next.source_revision, plan.post_source_revision);
        assert_eq!(next.archive_revision, revision(&original));
        assert_eq!(fs::read(&f.archive).unwrap(), original);
    }
}

#[test]
fn library_format_does_not_touch_completed_archive_outside_notes_root() {
    let outside = tempfile::tempdir().unwrap();
    let mut f = Fixture::new("- [x] Selected\n- [ ] Keep\n");
    f.archive = fs::canonicalize(outside.path()).unwrap().join("archive.md");
    let p = f.plan(1);
    f.commit(&p).unwrap();
    let original = fs::read(&f.archive).unwrap();
    crate::format_notes(&f.root, true).unwrap();
    assert_eq!(fs::read(&f.archive).unwrap(), original);
    assert_eq!(f.inspect(&p).phase, "completed");
}

#[test]
fn completed_history_can_move_and_append_without_relocating_old_operations() {
    let first = Fixture::new("- [x] First\n- [ ] Keep first\n");
    let old_plan = first.plan(1);
    first.commit(&old_plan).unwrap();
    let old_observation = first.inspect(&old_plan);
    let original = fs::read(&first.archive).unwrap();
    let mut second = Fixture::new("- [x] Second\n- [ ] Keep second\n");
    second.archive = second.root.join("relocated.md");
    fs::rename(&first.archive, &second.archive).unwrap();
    let decoded = Ledger::read(&checked(&second.archive).unwrap()).unwrap();
    assert_eq!(decoded.bytes().unwrap(), original);
    assert_eq!(decoded.records[0].receipt.plan, old_plan);

    let first_source = fs::read(&first.note).unwrap();
    let first_dashboard = fs::read(&first.dashboard).unwrap();
    let second_source = fs::read(&second.note).unwrap();
    let error = inspect_archive(
        &first.root,
        &first.dashboard,
        &second.archive,
        &old_plan.operation_id,
    )
    .unwrap_err()
    .to_string();
    assert!(error.contains("receipt destination mismatch"));
    assert!(
        inspect_archive(
            &second.root,
            &second.dashboard,
            &second.archive,
            &old_plan.operation_id
        )
        .is_err()
    );
    assert!(first.recover(&old_observation).is_err());
    // Retargeting submitted observation fields cannot grant authority over the old receipt.
    let mut retargeted = old_observation.clone();
    retargeted.plan.archive_path = utf8(&second.archive).unwrap();
    assert!(first.recover(&retargeted).is_err());
    retargeted.plan.notes_dir = utf8(&second.root).unwrap();
    retargeted.plan.dashboard_path = utf8(&second.dashboard).unwrap();
    assert!(second.recover(&retargeted).is_err());
    assert_eq!(fs::read(&second.archive).unwrap(), original);
    assert_eq!(fs::read(&first.note).unwrap(), first_source);
    assert_eq!(fs::read(&first.dashboard).unwrap(), first_dashboard);
    assert_eq!(fs::read(&second.note).unwrap(), second_source);
    assert!(!second.dashboard.exists());

    let fresh = second.plan(1);
    assert_eq!(fresh.archive_path, utf8(&second.archive).unwrap());
    assert_eq!(fresh.notes_dir, utf8(&second.root).unwrap());
    assert_eq!(fresh.dashboard_path, utf8(&second.dashboard).unwrap());
    assert_eq!(fresh.archive_revision, revision(&original));
    second.commit(&fresh).unwrap();
    let current = fs::read(&second.archive).unwrap();
    assert!(current.starts_with(&original));
    let ledger = Ledger::read(&checked(&second.archive).unwrap()).unwrap();
    assert_eq!(ledger.records.len(), 2);
    assert_eq!(ledger.records[0].receipt.plan, old_plan);
    assert_eq!(ledger.records[1].receipt.plan, fresh);
    assert_eq!(ledger.bytes().unwrap(), current);
    assert_eq!(second.inspect(&fresh).next_action, "none");
    assert!(
        inspect_archive(
            &first.root,
            &first.dashboard,
            &second.archive,
            &old_plan.operation_id
        )
        .is_err()
    );
    assert_eq!(fs::read(&first.note).unwrap(), first_source);
}

#[test]
fn moved_incomplete_history_cannot_append_or_recover_until_original_bindings_restored() {
    for fault in ["archive-copy-published", "archive-before-projection"] {
        let first = Fixture::new("- [x] First\n- [ ] Keep first\n");
        let p = first.plan(1);
        let dashboard = first.dashboard.clone();
        hook(move |stage| {
            if stage == fault {
                fs::write(&dashboard, "external edit").unwrap();
            }
        });
        assert!(first.commit(&p).is_err());
        clear();
        fs::remove_file(&first.dashboard).unwrap();
        let observation = first.inspect(&p);
        assert!(observation.blocked_reason.is_none());
        assert_eq!(
            observation.phase,
            if fault == "archive-copy-published" {
                "archiveCopied"
            } else {
                "sourcePruned"
            }
        );
        let original = fs::read(&first.archive).unwrap();
        let mut second = Fixture::new("- [x] Second\n- [ ] Keep second\n");
        second.archive = second.root.join("moved-incomplete.md");
        fs::rename(&first.archive, &second.archive).unwrap();
        // The codec can verify bytes, but this does not confer operation authority.
        assert_eq!(
            Ledger::read(&checked(&second.archive).unwrap())
                .unwrap()
                .bytes()
                .unwrap(),
            original
        );
        let task = second.todos().remove(0);
        let error = plan_archive(
            &second.root,
            &second.dashboard,
            &second.archive,
            &[task.id],
            &task.source_revision,
        )
        .unwrap_err()
        .to_string();
        assert!(error.contains("incomplete operation"));
        let source_before = fs::read(&first.note).unwrap();
        let other_before = fs::read(&second.note).unwrap();
        assert!(
            inspect_archive(
                &first.root,
                &first.dashboard,
                &second.archive,
                &p.operation_id
            )
            .is_err()
        );
        assert!(first.recover(&observation).is_err());
        let mut retargeted = observation.clone();
        retargeted.plan.archive_path = utf8(&second.archive).unwrap();
        assert!(first.recover(&retargeted).is_err());
        assert_eq!(fs::read(&second.archive).unwrap(), original);
        assert_eq!(fs::read(&first.note).unwrap(), source_before);
        assert_eq!(fs::read(&second.note).unwrap(), other_before);
        assert!(!first.dashboard.exists() && !second.dashboard.exists());
        fs::rename(&second.archive, &first.archive).unwrap();
        let fresh_inspection = first.inspect(&p);
        let result = first.recover(&fresh_inspection).unwrap();
        assert_eq!(result.source_pruned, fault == "archive-copy-published");
        assert_eq!(first.inspect(&p).next_action, "none");
    }
}

#[test]
fn relocated_completed_history_still_rejects_payload_corruption_and_hardlinks() {
    let first = Fixture::new("- [x] First\n- [ ] Keep first\n");
    let p = first.plan(1);
    first.commit(&p).unwrap();
    let mut second = Fixture::new("- [x] Second\n- [ ] Keep second\n");
    second.archive = second.root.join("moved.md");
    fs::rename(&first.archive, &second.archive).unwrap();
    let original = fs::read(&second.archive).unwrap();
    let mut damaged = original.clone();
    let offset = damaged
        .windows(b"- [x] First".len())
        .position(|w| w == b"- [x] First")
        .unwrap();
    damaged[offset + 6] = b'X';
    fs::write(&second.archive, &damaged).unwrap();
    let error = Ledger::read(&checked(&second.archive).unwrap())
        .err()
        .expect("corrupt moved payload must fail decoding");
    assert!(error.to_string().contains("payload integrity failure"));
    let task = second.todos().remove(0);
    assert!(
        plan_archive(
            &second.root,
            &second.dashboard,
            &second.archive,
            &[task.id.clone()],
            &task.source_revision
        )
        .is_err()
    );
    assert_eq!(fs::read(&second.archive).unwrap(), damaged);
    fs::write(&second.archive, &original).unwrap();
    #[cfg(unix)]
    {
        let alias = second.root.join("alias.md");
        fs::hard_link(&second.archive, &alias).unwrap();
        let error = plan_archive(
            &second.root,
            &second.dashboard,
            &second.archive,
            &[task.id],
            &task.source_revision,
        )
        .unwrap_err()
        .to_string();
        assert!(error.contains("hardlinked file refused"));
        assert_eq!(fs::read(&alias).unwrap(), original);
    }
    assert_eq!(fs::read(&second.archive).unwrap(), original);
}
