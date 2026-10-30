use super::*;

const SOURCE: &str = "---\nid: source\ndomains: [task-index]\ntask_active: true\ntask_scope: project\ntask_default_priority: p2\n---\n# Todo\n## Next\n- [x] Precision w:51.248178375505404\n- [ ] Keep\n";

fn fixture() -> (tempfile::TempDir, ArchivePlan) {
    let dir = tempfile::tempdir().unwrap();
    let root = fs::canonicalize(dir.path()).unwrap();
    let snapshot = Snapshot {
        path: root.join("archive.md"),
        bytes: Some(include_bytes!("v1-ledger.md").to_vec()),
    };
    let mut ledger = Ledger::read(&snapshot).unwrap();
    let p = &mut ledger.records[0].receipt.plan;
    p.notes_dir = utf8(&root).unwrap();
    p.dashboard_path = utf8(&root.join("dashboard.md")).unwrap();
    p.archive_path = utf8(&snapshot.path).unwrap();
    assert_eq!(revision(SOURCE.as_bytes()), p.source_revision);
    let p = p.clone();
    fs::write(root.join(&p.source_path), SOURCE).unwrap();
    fs::write(&p.archive_path, ledger.bytes().unwrap()).unwrap();
    (dir, p)
}

fn inspect(p: &ArchivePlan) -> ArchiveInspection {
    inspect_archive(
        &p.notes_dir,
        &p.dashboard_path,
        &p.archive_path,
        &p.operation_id,
    )
    .unwrap()
}

#[test]
fn historical_recovery_does_not_enable_runtime_aliases_or_new_plans() {
    let (_dir, p) = fixture();
    for recovered in [false, true] {
        if recovered {
            recover_archive(&p.notes_dir, &p.dashboard_path, &inspect(&p)).unwrap();
            let dashboard = fs::read_to_string(&p.dashboard_path).unwrap();
            assert!(dashboard.contains("- [ ] Keep <!-- mw-dashboard:v1:"));
            assert!(!dashboard.contains("Precision"));
            assert_eq!(inspect(&p).next_action, "none");
        }
        let source = Path::new(&p.notes_dir).join(&p.source_path);
        let original = fs::read_to_string(&source).unwrap();
        // Reject either legacy component independently, not just their combination.
        for bytes in [
            original.clone(),
            original
                .replace("domains: [task-index]", "domains: [task]")
                .replace("# Todo\n## Next", "## Todo\n### Next"),
            original.replace("# Todo\n## Next", "## Todo\n### Do Now"),
        ] {
            fs::write(&source, &bytes).unwrap();
            let before_archive = fs::read(&p.archive_path).unwrap();
            let before_dashboard = fs::read(&p.dashboard_path).ok();
            let expected = revision(bytes.as_bytes());
            let id = &p.items[0].task.id;
            assert!(list_active_task_index_todos(&p.notes_dir).is_err());
            assert!(get_active_task_index_todo(&p.notes_dir, id).is_err());
            assert!(
                plan_archive(
                    &p.notes_dir,
                    &p.dashboard_path,
                    &p.archive_path,
                    std::slice::from_ref(id),
                    &expected
                )
                .is_err()
            );
            assert!(commit_archive(&p.notes_dir, &p.dashboard_path, &p).is_err());
            assert!(
                toggle_task_index_todo(&p.notes_dir, &p.dashboard_path, id, &expected).is_err()
            );
            assert!(
                update_task_index_todos(
                    &p.notes_dir,
                    &p.dashboard_path,
                    TodoUpdateParams {
                        ids: vec![id.clone()],
                        expected_revision: expected,
                        title: Some("Changed".into()),
                        ..TodoUpdateParams::default()
                    }
                )
                .is_err()
            );
            assert!(sync_dashboard_from_task_index_notes(&p.notes_dir, &p.dashboard_path).is_err());
            assert!(
                refresh_dashboard_from_task_index_notes(&p.notes_dir, &p.dashboard_path).is_err()
            );
            assert_eq!(fs::read_to_string(&source).unwrap(), bytes);
            assert_eq!(fs::read(&p.archive_path).unwrap(), before_archive);
            assert_eq!(fs::read(&p.dashboard_path).ok(), before_dashboard);
        }
        fs::write(source, original).unwrap();
    }
}

#[test]
fn historical_post_state_projects_survivors_without_resolving_old_positional_ids() {
    for phase in ["archiveCopied", "sourcePruned", "completed"] {
        let (_dir, p) = fixture();
        let source = Path::new(&p.notes_dir).join(&p.source_path);
        let post = SOURCE.replace("- [x] Precision w:51.248178375505404\n", "");
        assert_eq!(revision(post.as_bytes()), p.post_source_revision);
        fs::write(&source, &post).unwrap();
        let snapshot = checked(Path::new(&p.archive_path)).unwrap();
        let mut ledger = Ledger::read(&snapshot).unwrap();
        ledger.records[0].receipt.phase = phase.into();
        fs::write(&p.archive_path, ledger.bytes().unwrap()).unwrap();
        let observed = inspect(&p);
        assert_eq!(
            observed.next_action, "refreshOnly",
            "{:?}",
            observed.blocked_reason
        );
        let result = recover_archive(&p.notes_dir, &p.dashboard_path, &observed).unwrap();
        assert!(!result.source_pruned);
        assert!(
            fs::read_to_string(&p.dashboard_path)
                .unwrap()
                .contains("- [ ] Keep <!-- mw-dashboard:v1:")
        );
        assert_eq!(fs::read_to_string(source).unwrap(), post);
        assert_eq!(inspect(&p).next_action, "none");
    }
}

#[test]
fn historical_recovery_blocks_stale_bytes_and_unbound_legacy_sources() {
    for post in [false, true] {
        let (_dir, p) = fixture();
        if post {
            recover_archive(&p.notes_dir, &p.dashboard_path, &inspect(&p)).unwrap();
        }
        let observed = inspect(&p);
        let source = Path::new(&p.notes_dir).join(&p.source_path);
        let bytes = fs::read_to_string(&source).unwrap() + "\n";
        fs::write(&source, &bytes).unwrap();
        let archive = fs::read(&p.archive_path).unwrap();
        let dashboard = fs::read(&p.dashboard_path).ok();
        assert!(recover_archive(&p.notes_dir, &p.dashboard_path, &observed).is_err());
        let changed = inspect(&p);
        assert_eq!(changed.source_state, "changed");
        assert!(changed.blocked_reason.is_some());
        assert!(recover_archive(&p.notes_dir, &p.dashboard_path, &changed).is_err());
        assert_eq!(fs::read_to_string(source).unwrap(), bytes);
        assert_eq!(fs::read(&p.archive_path).unwrap(), archive);
        assert_eq!(fs::read(&p.dashboard_path).ok(), dashboard);
    }
    let (_dir, p) = fixture();
    fs::write(
        Path::new(&p.notes_dir).join("unbound.md"),
        SOURCE.replace("id: source", "id: other"),
    )
    .unwrap();
    let blocked = inspect(&p);
    assert!(blocked.blocked_reason.unwrap().contains("unbound.md"));
    assert_eq!(blocked.next_action, "none");
}

#[test]
fn historical_v1_recovery_preserves_stable_ids_metadata_and_known_section_contexts() {
    for (todo, section, newline) in [("# Todo", "##", "\n"), ("## Todo", "###", "\r\n")] {
        let (_dir, mut p) = fixture();
        let source = Path::new(&p.notes_dir).join(&p.source_path);
        let content = format!(
            "---\nid: source\ndomains: [task-index]\ntask_active: true\ntask_scope: project\ntask_default_priority: p2\n---\n{todo}\n{section} nExT\n- [x] Selected\n  - p1 e:s\n  <!-- mw-task:v1:0123456789abcdef0123456789abcdef -->\n- [ ] Keep\n  <!-- mw-task:v1:fedcba9876543210fedcba9876543210 -->\n{section} Waiting\n- [ ] Wait\n{section} Inbox\n- [ ] Triage\n{section} Blocked\n- [ ] Blocked\n{section} Done\n- [x] Previous\n"
        ).replace('\n', newline);
        fs::write(&source, &content).unwrap();
        p.source_revision = revision(content.as_bytes());
        let (todos, _, _) = archive_scan(Path::new(&p.notes_dir), Some(&p), None, false).unwrap();
        let selected = &todos[0];
        let (start, length) = span(&content, selected).unwrap();
        let payload = content.as_bytes()[start..start + length].to_vec();
        let mut post = content.as_bytes()[..start].to_vec();
        post.extend_from_slice(&content.as_bytes()[start + length..]);
        p.post_source_revision = revision(&post);
        p.raw_byte_count = length;
        p.items = vec![ArchiveItem {
            task: selected.clone().into(),
            byte_start: start,
            byte_length: length,
            block_revision: revision(&payload),
        }];
        let mut ledger = Ledger::read(&checked(Path::new(&p.archive_path)).unwrap()).unwrap();
        ledger.records[0].receipt.plan = p.clone();
        ledger.records[0].payload = payload.clone();
        fs::write(&p.archive_path, ledger.bytes().unwrap()).unwrap();
        let observed = inspect(&p);
        assert_eq!(
            observed.next_action, "resumePrune",
            "{:?}",
            observed.blocked_reason
        );
        recover_archive(&p.notes_dir, &p.dashboard_path, &observed).unwrap();
        assert_eq!(fs::read(&source).unwrap(), post);
        let dashboard = fs::read_to_string(&p.dashboard_path).unwrap();
        assert!(dashboard.contains("- [ ] Keep <!-- mw-dashboard:v1:"));
        for hidden in ["Selected", "Wait", "Triage", "Blocked", "Previous"] {
            assert!(!dashboard.contains(hidden));
        }
        let completed = Ledger::read(&checked(Path::new(&p.archive_path)).unwrap()).unwrap();
        assert_eq!(completed.records[0].receipt.plan, p);
        assert_eq!(completed.records[0].payload, payload);
        assert_eq!(inspect(&p).next_action, "none");
    }
}
