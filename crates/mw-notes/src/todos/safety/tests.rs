use super::*;

struct Fixture {
    root: tempfile::TempDir,
    note: PathBuf,
    dashboard: PathBuf,
}
impl Fixture {
    fn new(tasks: &str) -> Self {
        let root = tempfile::tempdir().unwrap();
        let note = root.path().join("tasks.md");
        let dashboard = root.path().join("dashboard.md");
        fs::write(&note, format!("---\nid: source\ndomains: [task-index]\ntask_active: true\n---\n# Todo\n## Next\n{tasks}")).unwrap();
        Self {
            root,
            note,
            dashboard,
        }
    }
    fn list(&self) -> Vec<TaskIndexTodo> {
        list_active_task_index_todos(self.root.path()).unwrap().0
    }
    fn sync(&self) -> io::Result<SyncStats> {
        project(self.root.path(), &self.dashboard, true, false)
    }
    fn toggle(&self, todo: &TaskIndexTodo) -> io::Result<(TaskIndexTodo, bool)> {
        toggle_task_index_todo(
            self.root.path(),
            &self.dashboard,
            &todo.id,
            &todo.source_revision,
        )
    }
    fn identify(&self, todo: &TaskIndexTodo) -> TaskIndexTodo {
        identify_task_index_todo(
            self.root.path(),
            &self.dashboard,
            &todo.id,
            &todo.source_revision,
        )
        .unwrap()
    }
    fn edit_dashboard(&self) {
        fs::write(
            &self.dashboard,
            fs::read_to_string(&self.dashboard)
                .unwrap()
                .replacen("- [ ]", "- [x]", 1),
        )
        .unwrap();
    }
    fn bytes(&self) -> (Vec<u8>, Option<Vec<u8>>) {
        (
            fs::read(&self.note).unwrap(),
            fs::read(&self.dashboard).ok(),
        )
    }

    fn operation(&self, operation: &str) -> io::Result<()> {
        match operation {
            "sync" => self.sync().map(|_| ()),
            "refresh" => refresh_dashboard_from_task_index_notes(self.root.path(), &self.dashboard)
                .map(|_| ()),
            "discard" => {
                refresh_dashboard_discarding_edits(self.root.path(), &self.dashboard).map(|_| ())
            }
            _ => {
                let todo = list_active_task_index_todos(self.root.path())?
                    .0
                    .into_iter()
                    .next()
                    .ok_or_else(|| invalid("test operation requires a task"))?;
                match operation {
                    "toggle" => self.toggle(&todo).map(|_| ()),
                    "identify" => identify_task_index_todo(
                        self.root.path(),
                        &self.dashboard,
                        &todo.id,
                        &todo.source_revision,
                    )
                    .map(|_| ()),
                    "update" => update_task_index_todos(
                        self.root.path(),
                        &self.dashboard,
                        TodoUpdateParams {
                            ids: vec![todo.id],
                            expected_revision: todo.source_revision,
                            priority: Some("p1".into()),
                            ..Default::default()
                        },
                    )
                    .map(|_| ()),
                    _ => panic!("unknown operation: {operation}"),
                }
            }
        }
    }
}

#[test]
fn all_markdown_checkbox_forms_are_nested_barriers_and_unsupported_roots_fail_closed() {
    for bullet in ["-", "*", "+", "1.", "1)", "42.", "42)"] {
        for variant in 0..4 {
            let parent_marker = if variant == 2 {
                "  <!-- mw-task:v1:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb -->\n"
            } else {
                ""
            };
            let child_marker = if variant > 0 {
                "    <!-- mw-task:v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n"
            } else {
                ""
            };
            let other = if variant == 3 {
                format!("  {bullet} [x] Another child\n")
            } else {
                String::new()
            };
            let f = Fixture::new(&format!(
                "- [ ] Parent\n{parent_marker}  {bullet} [ ] Child\n{child_marker}{other}"
            ));
            let before = f.bytes();
            let err = list_active_task_index_todos(f.root.path())
                .unwrap_err()
                .to_string();
            assert!(
                err.contains("tasks.md") && err.contains("nested checkbox"),
                "{bullet}: {err}"
            );
            assert!(f.sync().is_err());
            assert!(refresh_dashboard_from_task_index_notes(f.root.path(), &f.dashboard).is_err());
            assert!(
                toggle_task_index_todo(
                    f.root.path(),
                    &f.dashboard,
                    "source:8:0",
                    &revision(&before.0)
                )
                .is_err()
            );
            assert!(
                identify_task_index_todo(
                    f.root.path(),
                    &f.dashboard,
                    "source:8:0",
                    &revision(&before.0)
                )
                .is_err()
            );
            assert!(
                update_task_index_todos(
                    f.root.path(),
                    &f.dashboard,
                    TodoUpdateParams {
                        ids: vec!["source:8:0".into()],
                        expected_revision: revision(&before.0),
                        priority: Some("p1".into()),
                        ..Default::default()
                    }
                )
                .is_err()
            );
            assert_eq!(f.bytes(), before);
            if variant > 0 {
                let masked = mask_task_examples(&fs::read_to_string(&f.note).unwrap()).unwrap();
                assert!(
                    identities(
                        &masked,
                        &[ParsedTask {
                            line: 8,
                            ..Default::default()
                        }]
                    )
                    .is_err()
                );
            }
        }
        let f = Fixture::new(&format!(
            "- [ ] Parent\n  ```md\n  {bullet} [ ] Example\n    <!-- mw-task:v1:BAD -->\n  ```\n"
        ));
        assert_eq!(f.list().len(), 1);
        f.toggle(&f.list()[0]).unwrap();
        if !matches!(bullet, "-" | "*") {
            let f = Fixture::new(&format!("{bullet} [ ] Unsupported root\n"));
            assert!(
                list_active_task_index_todos(f.root.path())
                    .unwrap_err()
                    .to_string()
                    .contains("unsupported checkbox syntax")
            );
        }
    }
}

#[test]
fn task_authority_uses_only_top_level_yaml_not_description_operational_keys() {
    for newline in ["\n", "\r\n"] {
        for scalar in ["|", ">"] {
            for active in [false, true] {
                let f = Fixture::new("");
                let prefix = format!("---\nid: real-source\ndomains: [task-index]\ntask_active: {active}\ntask_area: Code\ntask_default_priority: p2\ntask_default_energy: small\ntitle: Real title\ntask_scope: project\ndescription: {scalar}\n  id: example\n  task_active: {}\n  domains: [task-index, fake]\n  task_area: Music\n  task_default_priority: p5\n  task_default_energy: xl\n  title: Wrong title\n  task_scope: fake\n  ## Todo\n  ### Next\n  - [ ] Example\n    <!-- mw-task:v1:BAD -->\n  ---\n  After\n---\n", !active).replace('\n', newline);
                fs::write(
                    &f.note,
                    prefix.clone() + &"# Todo\n## Next\n- [ ] Real\n".replace('\n', newline),
                )
                .unwrap();
                let before = f.bytes();
                let todos = f.list();
                if !active {
                    assert!(todos.is_empty());
                    assert!(
                        identify_task_index_todo(
                            f.root.path(),
                            &f.dashboard,
                            "example:1:0",
                            &revision(&before.0)
                        )
                        .is_err()
                    );
                    assert_eq!(f.bytes(), before);
                    continue;
                }
                assert_eq!(todos.len(), 1);
                for todo in [
                    &todos[0],
                    &f.identify(&todos[0]),
                    &f.toggle(&f.list()[0]).unwrap().0,
                ] {
                    assert_eq!(todo.source_id, "real-source");
                    assert_eq!(todo.area, "Code");
                    assert_eq!(todo.note_title, "Real title");
                    assert_eq!(todo.task_scope, "project");
                    assert_eq!(todo.metadata.priority, "p2");
                    assert_eq!(todo.metadata.energy, "small");
                }
                assert!(fs::read(&f.note).unwrap().starts_with(prefix.as_bytes()));
            }
        }
    }
}

#[test]
fn task_yaml_preserves_plain_domain_forms_and_rejects_unsupported_operational_types() {
    let inactive = Fixture::new("- [ ] Real\n");
    fs::write(
        &inactive.note,
        fs::read_to_string(&inactive.note)
            .unwrap()
            .replace("domains: [task-index]", "domains: [glossary]")
            .replace(
                "---\n# Todo",
                "description: |\n  domains: [task-index]\n  task_active: true\n---\n# Todo",
            ),
    )
    .unwrap();
    assert!(inactive.list().is_empty());
    for domains in [
        "task-index",
        "task-index, glossary",
        "[task-index, glossary]",
        "\n  - task-index\n  - glossary",
    ] {
        let f = Fixture::new("- [ ] Real\n");
        fs::write(
            &f.note,
            fs::read_to_string(&f.note)
                .unwrap()
                .replace("domains: [task-index]", &format!("domains: {domains}"))
                .replace("task_active: true", "task_active: yes"),
        )
        .unwrap();
        assert_eq!(f.list().len(), 1);
        f.toggle(&f.list()[0]).unwrap();
    }
    for field in [
        "id: [bad]",
        "title: {bad: mapping}",
        "task_scope: [bad]",
        "task_active: [true]",
        "task_area: {bad: mapping}",
        "task_default_priority: [p1]",
        "task_default_energy: [small]",
        "domains: [task-index, 42]",
    ] {
        let f = Fixture::new("- [ ] Real\n");
        let key = field.split(':').next().unwrap();
        let original = fs::read_to_string(&f.note).unwrap();
        let lines = original
            .lines()
            .filter(|line| !line.starts_with(&format!("{key}:")))
            .collect::<Vec<_>>()
            .join("\n");
        fs::write(
            &f.note,
            lines.replacen("---\n", &format!("---\n{field}\n"), 1),
        )
        .unwrap();
        let before = f.bytes();
        assert!(
            list_active_task_index_todos(f.root.path()).is_err(),
            "{field}"
        );
        assert!(f.sync().is_err());
        assert_eq!(f.bytes(), before);
    }
}

#[test]
fn layout_integrity_rejects_row_membership_order_area_and_manifest_edits_before_any_mutation() {
    for edit in [
        "delete",
        "reorder",
        "move",
        "metadata",
        "header",
        "digest",
        "missing",
        "duplicate",
    ] {
        for operation in ["sync", "refresh", "toggle", "update", "identify"] {
            let f = Fixture::new(
                "- [ ] First\n  - area: Code\n- [ ] Second\n  - area: Code\n- [ ] Third\n  - area: Reading\n",
            );
            f.identify(&f.list()[0]);
            f.sync().unwrap();
            let content = fs::read_to_string(&f.dashboard)
                .unwrap()
                .replace("## Reading\n- [ ]", "## Reading\n- [x]");
            let mut lines: Vec<_> = content.lines().map(str::to_string).collect();
            let rows: Vec<_> = lines
                .iter()
                .enumerate()
                .filter(|(_, l)| l.starts_with("- ["))
                .map(|(i, _)| i)
                .collect();
            let manifest = lines.iter().position(|l| layout::is_marker(l)).unwrap();
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
                "metadata" => {
                    lines[rows[0]] =
                        lines[rows[0]].replace("mw-dashboard:v1:", "mw-dashboard:v1:00")
                }
                "header" => {}
                "digest" => {
                    let mut bad = lines[manifest].clone();
                    let index = bad.find("sha256:").unwrap() + 7;
                    let other = if &bad[index..index + 1] == "0" {
                        "1"
                    } else {
                        "0"
                    };
                    bad.replace_range(index..index + 1, other);
                    lines[manifest] = bad;
                }
                "missing" => {
                    lines.remove(manifest);
                }
                "duplicate" => lines.insert(manifest, lines[manifest].clone()),
                _ => unreachable!(),
            }
            let content = lines.join("\n");
            let content = if edit == "header" {
                content.replace(HEADER, "<!-- mw-dashboard:v9 -->")
            } else {
                content
            };
            fs::write(&f.dashboard, content).unwrap();
            let before = f.bytes();
            assert!(f.operation(operation).is_err(), "{edit} {operation}");
            assert_eq!(f.bytes(), before, "{edit} {operation}");
            f.operation("discard").unwrap();
            assert_eq!(fs::read(&f.note).unwrap(), before.0);
            assert!(
                fs::read_to_string(&f.dashboard)
                    .unwrap()
                    .contains("mw-dashboard-layout:v1:sha256:")
            );
        }
    }
}

#[test]
fn layout_digest_allows_only_checkbox_changes_and_line_ending_or_blank_line_variation() {
    let f = Fixture::new("- [ ] First\n- [ ] Second\n");
    f.sync().unwrap();
    let content = fs::read_to_string(&f.dashboard).unwrap();
    let original_marker = content
        .lines()
        .find(|l| layout::is_marker(l))
        .unwrap()
        .to_string();
    let edited = content
        .replacen("- [ ]", "- [X]", 1)
        .replace('\n', "\r\n\r\n");
    // Keep frontmatter bytes intact: canonicalization applies only to the body.
    let prefix = extract_frontmatter_prefix(&content).unwrap().unwrap();
    let edited_prefix = extract_frontmatter_prefix(&edited).unwrap().unwrap();
    fs::write(
        &f.dashboard,
        prefix.to_string() + &edited[edited_prefix.len()..],
    )
    .unwrap();
    assert_eq!(f.sync().unwrap().source_writebacks, 1);
    assert!(f.list()[0].done);
    let canonical = fs::read_to_string(&f.dashboard).unwrap();
    assert_ne!(
        canonical.lines().find(|l| layout::is_marker(l)).unwrap(),
        original_marker
    );
    fs::write(
        &f.note,
        fs::read_to_string(&f.note)
            .unwrap()
            .replace("Second", "Retitled"),
    )
    .unwrap();
    let source = fs::read(&f.note).unwrap();
    f.sync().unwrap();
    assert_eq!(fs::read(&f.note).unwrap(), source);
    assert!(
        fs::read_to_string(&f.dashboard)
            .unwrap()
            .contains("Retitled")
    );
    let f = Fixture::new("");
    let prefix = "---\ncustom: preserved\ntitle: [arbitrary, dashboard, metadata]\ndomains: {custom: metadata}\n---\n";
    fs::write(&f.dashboard, format!("{prefix}{HEADER}\n")).unwrap();
    f.sync().unwrap();
    let generated = fs::read_to_string(&f.dashboard).unwrap();
    assert!(generated.starts_with(prefix));
    assert!(generated.contains("mw-dashboard-layout:v1:sha256:"));
    f.operation("refresh").unwrap();
}

#[test]
fn missing_notes_root_cannot_be_created_by_lock_acquisition() {
    let parent = tempfile::tempdir().unwrap();
    let root = parent.path().join("missing/notes");
    assert!(project(&root, &parent.path().join("dashboard.md"), false, true).is_err());
    assert!(!parent.path().join("missing").exists());
    let file = parent.path().join("file");
    fs::write(&file, "unchanged").unwrap();
    assert!(Lock::acquire(&file).is_err());
    assert_eq!(fs::read_to_string(file).unwrap(), "unchanged");
}

#[test]
fn unsupported_dashboard_body_edits_block_every_operation_until_explicit_discard() {
    for extra in ["Personal prose\n", "## Custom heading\n", "# Other title\n"] {
        for operation in ["sync", "refresh", "toggle", "update", "identify"] {
            for marked in [false, true] {
                let f = Fixture::new("- [ ] One\n");
                if marked {
                    f.identify(&f.list()[0]);
                }
                f.sync().unwrap();
                // Include a pending checkbox on sync to prove body validation precedes source writes.
                if operation == "sync" {
                    f.edit_dashboard();
                }
                fs::write(
                    &f.dashboard,
                    fs::read_to_string(&f.dashboard).unwrap() + extra,
                )
                .unwrap();
                let before = f.bytes();
                assert!(
                    f.operation(operation)
                        .unwrap_err()
                        .to_string()
                        .contains("body edit"),
                    "{operation}"
                );
                assert_eq!(f.bytes(), before, "{operation}");
                f.operation("discard").unwrap();
                assert_eq!(fs::read(&f.note).unwrap(), before.0);
                assert!(!fs::read_to_string(&f.dashboard).unwrap().contains(extra));
            }
        }
    }
}

#[test]
fn complete_scalar_frontmatter_prefix_is_preserved_exactly_in_lf_and_crlf() {
    for newline in ["\n", "\r\n"] {
        for scalar in ["|", ">"] {
            let f = Fixture::new("- [ ] One\n");
            f.sync().unwrap();
            let prefix = format!("---\nid: dashboard\ndescription: {scalar}\n  Before\n  ---\n  After\ncustomfield: keep me\nexample: |\n  - [x] not a dashboard row\n---\n").replace('\n', newline);
            let original = fs::read_to_string(&f.dashboard).unwrap();
            let old_prefix = extract_frontmatter_prefix(&original).unwrap().unwrap();
            fs::write(&f.dashboard, prefix.clone() + &original[old_prefix.len()..]).unwrap();
            for operation in [
                "sync", "refresh", "toggle", "update", "identify", "identify", "discard",
            ] {
                f.operation(operation).unwrap();
                let content = fs::read_to_string(&f.dashboard).unwrap();
                assert_eq!(
                    extract_frontmatter_prefix(&content).unwrap(),
                    Some(prefix.as_str()),
                    "{newline:?} {scalar} {operation}"
                );
                assert!(content.as_bytes().starts_with(prefix.as_bytes()));
            }
        }
    }
    // A closing delimiter without a final newline is still preserved verbatim.
    assert_eq!(
        extract_frontmatter_prefix("---\ncustom: yes\n---").unwrap(),
        Some("---\ncustom: yes\n---")
    );
}

#[test]
fn malformed_frontmatter_is_preserved_even_with_explicit_body_discard() {
    for prefix in [
        "---\ncustom: missing close\n",
        "---\ncustom: [broken\n---\n",
        "---\ncustom: value\n  ---\n",
        " ---\ncustom: value\n---\n",
    ] {
        for operation in ["sync", "refresh", "discard", "toggle", "update", "identify"] {
            let f = Fixture::new("- [ ] One\n");
            f.sync().unwrap();
            let original = fs::read_to_string(&f.dashboard).unwrap();
            let body = &original[extract_frontmatter_prefix(&original)
                .unwrap()
                .unwrap()
                .len()..];
            fs::write(&f.dashboard, prefix.to_string() + body).unwrap();
            let before = f.bytes();
            assert!(
                f.operation(operation)
                    .unwrap_err()
                    .to_string()
                    .contains("frontmatter"),
                "{operation}"
            );
            assert_eq!(f.bytes(), before, "{operation}");
        }
    }
}

#[test]
fn nested_checkbox_tasks_never_lend_their_identity_to_the_parent() {
    let uid = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
    for tasks in [
        format!("- [ ] Parent\n  - [ ] Child\n    <!-- mw-task:v1:{uid} -->\n"),
        format!(
            "- [ ] Parent\n  <!-- mw-task:v1:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb -->\n  - [ ] Child\n    <!-- mw-task:v1:{uid} -->\n"
        ),
        format!(
            "- [ ] Parent\n  - [ ] Child\n    <!-- mw-task:v1:{uid} -->\n  - [ ] Another child\n"
        ),
        "- [ ] Parent\n  - [ ] Unmarked child\n".to_string(),
    ] {
        let f = Fixture::new(&tasks);
        let before = f.bytes();
        assert!(
            list_active_task_index_todos(f.root.path())
                .unwrap_err()
                .to_string()
                .contains("nested checkbox")
        );
        assert!(get_active_task_index_todo(f.root.path(), &format!("task:{uid}")).is_err());
        assert!(f.sync().is_err());
        assert!(refresh_dashboard_discarding_edits(f.root.path(), &f.dashboard).is_err());
        assert!(
            toggle_task_index_todo(
                f.root.path(),
                &f.dashboard,
                "source:8:0",
                &revision(&before.0)
            )
            .is_err()
        );
        assert!(
            identify_task_index_todo(
                f.root.path(),
                &f.dashboard,
                "source:8:0",
                &revision(&before.0)
            )
            .is_err()
        );
        assert!(
            update_task_index_todos(
                f.root.path(),
                &f.dashboard,
                TodoUpdateParams {
                    ids: vec!["source:8:0".into()],
                    expected_revision: revision(&before.0),
                    priority: Some("p1".into()),
                    ..Default::default()
                }
            )
            .is_err()
        );
        assert_eq!(f.bytes(), before);
        // Defense in depth if a parser ever supplies only the parent again.
        let fallback = vec![ParsedTask {
            line: 8,
            ..Default::default()
        }];
        if tasks.contains("mw-task:") {
            let masked = mask_task_examples(&fs::read_to_string(&f.note).unwrap()).unwrap();
            assert!(identities(&masked, &fallback).is_err());
        }
    }
}

#[test]
fn fenced_nested_examples_and_ordinary_children_remain_supported() {
    let f = Fixture::new(
        "- [ ] Parent\n  ordinary child prose\n  - p1 e:s\n  ```markdown\n  - [ ] Child example\n    <!-- mw-task:v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n  ```\n",
    );
    let todos = f.list();
    assert_eq!(todos.len(), 1);
    assert_eq!(todos[0].identity_kind, "positional");
    assert_eq!(todos[0].metadata.priority, "p1");
    let marked = f.identify(&todos[0]);
    assert_ne!(
        marked.stable_id.as_deref(),
        Some("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
    );
    f.toggle(&marked).unwrap();
    let content = fs::read_to_string(&f.note).unwrap();
    assert!(
        content
            .contains("  ordinary child prose\n  - p1 e:s\n  ```markdown\n  - [ ] Child example")
    );
}

#[test]
fn frontmatter_task_examples_are_never_operational_or_mutated() {
    for newline in ["\n", "\r\n"] {
        for scalar in ["|", ">"] {
            for marker in [
                "",
                "    <!-- mw-task:v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n",
                "    <!-- mw-task:v1:BAD -->\n",
            ] {
                let f = Fixture::new("");
                let prefix = format!("---\nid: source\ndomains: [task-index]\ntask_active: true\ndescription: {scalar}\n  Before\n  ## Todo\n  ### Next\n  - [ ] Example\n{marker}  ---\n  After\n---\n").replace('\n', newline);
                let body = "# Todo\n## Next\n- [ ] Real\n  ordinary child\n```markdown\n# Todo\n- [ ] Fenced example\n  <!-- mw-task:v1:BAD -->\n```\n".replace('\n', newline);
                let original = prefix.clone() + &body;
                fs::write(&f.note, &original).unwrap();
                let todos = f.list();
                assert_eq!(todos.len(), 1);
                let todo = &todos[0];
                assert_eq!(todo.text, "Real");
                assert_eq!(todo.line, prefix.lines().count() + 3);
                assert_eq!(todo.order, 0);
                assert_eq!(todo.identity_kind, "positional");
                assert_eq!(todo.source_revision, revision(original.as_bytes()));
                assert_eq!(fs::read(&f.note).unwrap(), original.as_bytes());
                for fake_id in ["source:9:0", "task:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] {
                    let before = f.bytes();
                    assert!(
                        identify_task_index_todo(
                            f.root.path(),
                            &f.dashboard,
                            fake_id,
                            &todo.source_revision
                        )
                        .is_err()
                    );
                    assert!(
                        toggle_task_index_todo(
                            f.root.path(),
                            &f.dashboard,
                            fake_id,
                            &todo.source_revision
                        )
                        .is_err()
                    );
                    assert_eq!(f.bytes(), before);
                }
                let marked = f.identify(todo);
                assert_eq!(f.list().len(), 1);
                let after_identify = fs::read_to_string(&f.note).unwrap();
                assert_eq!(
                    extract_frontmatter_prefix(&after_identify).unwrap(),
                    Some(prefix.as_str())
                );
                assert!(after_identify.ends_with(&body[body.find("  ordinary child").unwrap()..]));
                f.toggle(&marked).unwrap();
                let after_toggle = fs::read_to_string(&f.note).unwrap();
                assert_eq!(
                    after_toggle,
                    after_identify.replacen("- [ ] Real", "- [x] Real", 1)
                );
                assert!(after_toggle.as_bytes().starts_with(prefix.as_bytes()));
            }
        }
    }
}

#[test]
fn malformed_active_frontmatter_fails_task_mask_without_source_writes() {
    let f = Fixture::new("- [ ] Real\n");
    let content = fs::read_to_string(&f.note).unwrap().replacen(
        "task_active: true",
        "task_active: true\ndescription: [broken",
        1,
    );
    fs::write(&f.note, &content).unwrap();
    let before = f.bytes();
    assert!(
        list_active_task_index_todos(f.root.path())
            .unwrap_err()
            .to_string()
            .contains("frontmatter")
    );
    assert!(f.sync().is_err());
    assert_eq!(f.bytes(), before);
    assert!(mask_task_examples("---\ndescription: no closing fence\n  ---\n").is_err());
}

#[test]
fn identity_survives_insert_reorder_title_metadata_and_move() {
    let f = Fixture::new("- [ ] One\n- [ ] Two\n");
    let before = fs::read(&f.note).unwrap();
    let todo = f.list().remove(0);
    assert_eq!(todo.identity_kind, "positional");
    assert_eq!(fs::read(&f.note).unwrap(), before);
    let marked = f.identify(&todo);
    assert_eq!(marked.identity_kind, "stable");
    assert_eq!(marked.stable_id.as_ref().unwrap().len(), 32);
    let snapshot = f.bytes();
    assert_eq!(f.identify(&marked), marked);
    assert_eq!(f.bytes(), snapshot);
    assert!(
        identify_task_index_todo(
            f.root.path(),
            &f.dashboard,
            &marked.id,
            &todo.source_revision
        )
        .is_err()
    );
    let uid = marked.stable_id.as_ref().unwrap();
    fs::write(&f.note, format!("---\nid: source\ndomains: [task-index]\ntask_active: true\n---\n# Todo\n## Next\n- [ ] New\n- [ ] Two\n- [ ] Retitled\n  - p1\n  <!-- mw-task:v1:{uid} -->\n")).unwrap();
    fs::rename(&f.note, f.root.path().join("moved.md")).unwrap();
    let current = get_active_task_index_todo(f.root.path(), &marked.id).unwrap();
    assert_eq!(current.id, marked.id);
    assert_eq!(current.text, "Retitled");
    assert_eq!(current.metadata.priority, "p1");
    assert_ne!(current.source_revision, marked.source_revision);
}

#[test]
fn malformed_duplicate_and_orphan_markers_fail_closed_but_fences_are_examples() {
    let uid = "a".repeat(32);
    for task in [
        format!("- [ ] One\n<!-- mw-task:v1:{uid} -->"),
        format!("- [ ] One\n  <!-- mw-task:v1:BAD -->"),
        format!("- [ ] One\n  <!-- mw-task:v1:{uid} -->\n  <!-- mw-task:v1:{uid} -->"),
        format!("- [ ] One\n  <!-- mw-task:v1:{uid} -->\n- [ ] Two\n  <!-- mw-task:v1:{uid} -->"),
    ] {
        let f = Fixture::new(&task);
        let before = f.bytes();
        assert!(list_active_task_index_todos(f.root.path()).is_err());
        assert!(f.sync().is_err());
        assert_eq!(f.bytes(), before);
    }
    let f =
        Fixture::new("```md\n# Todo\n- [ ] example\n  <!-- mw-task:v1:BAD -->\n```\n- [ ] Real\n");
    assert_eq!(f.list().len(), 1);
    assert_eq!(f.list()[0].text, "Real");
}

#[test]
fn duplicate_source_ids_even_without_tasks_and_global_stable_ids_are_errors() {
    let f = Fixture::new("- [ ] One\n");
    fs::write(
        f.root.path().join("other.md"),
        fs::read_to_string(&f.note)
            .unwrap()
            .replace("- [ ] One\n", ""),
    )
    .unwrap();
    assert!(
        list_active_task_index_todos(f.root.path())
            .unwrap_err()
            .to_string()
            .contains("duplicate active source")
    );
    let uid = "b".repeat(32);
    fs::write(
        &f.note,
        fs::read_to_string(&f.note).unwrap() + &format!("  <!-- mw-task:v1:{uid} -->\n"),
    )
    .unwrap();
    fs::write(
        f.root.path().join("other.md"),
        fs::read_to_string(&f.note)
            .unwrap()
            .replace("id: source", "id: other"),
    )
    .unwrap();
    assert!(
        list_active_task_index_todos(f.root.path())
            .unwrap_err()
            .to_string()
            .contains("duplicate global")
    );
}

#[test]
fn additive_fields_deserialize_old_json() {
    let f = Fixture::new("- [ ] One");
    let mut json = serde_json::to_value(&f.list()[0]).unwrap();
    for field in ["stableId", "identityKind", "sourceRevision"] {
        json.as_object_mut().unwrap().remove(field);
    }
    let todo: TaskIndexTodo = serde_json::from_value(json).unwrap();
    assert_eq!(todo.identity_kind, "positional");
    assert_eq!(todo.stable_id, None);
    assert!(todo.source_revision.is_empty());
}

#[test]
fn stale_positional_id_now_points_to_another_checkbox() {
    let f = Fixture::new("- [ ] One\n- [ ] Two\n");
    let todo = f.list().remove(0);
    fs::write(
        &f.note,
        fs::read_to_string(&f.note)
            .unwrap()
            .replace("- [ ] One", "- [ ] New\n- [ ] One"),
    )
    .unwrap();
    let before = f.bytes();
    assert!(f.toggle(&todo).unwrap_err().to_string().contains("stale"));
    assert_eq!(f.bytes(), before);
}

#[test]
fn cross_source_bulk_and_duplicate_requests_preflight() {
    let f = Fixture::new("- [ ] One\n");
    fs::write(
        f.root.path().join("other.md"),
        fs::read_to_string(&f.note)
            .unwrap()
            .replace("id: source", "id: other"),
    )
    .unwrap();
    let todos = f.list();
    for ids in [
        todos.iter().map(|t| t.id.clone()).collect(),
        vec![todos[0].id.clone(), todos[0].id.clone()],
    ] {
        let before = f.bytes();
        assert!(
            update_task_index_todos(
                f.root.path(),
                &f.dashboard,
                TodoUpdateParams {
                    ids,
                    expected_revision: todos[0].source_revision.clone(),
                    priority: Some("p1".into()),
                    ..Default::default()
                }
            )
            .is_err()
        );
        assert_eq!(f.bytes(), before);
    }
}

#[test]
fn bulk_returns_all_targets_after_line_shifts_and_preserves_markers() {
    let f = Fixture::new("- [ ] One\n- [ ] Two\n");
    let first = f.identify(&f.list()[0]);
    let todos = f.list();
    let result = update_task_index_todos(
        f.root.path(),
        &f.dashboard,
        TodoUpdateParams {
            ids: todos.iter().map(|t| t.id.clone()).collect(),
            expected_revision: todos[0].source_revision.clone(),
            priority: Some("p1".into()),
            ..Default::default()
        },
    )
    .unwrap();
    assert_eq!(result.len(), 2);
    assert_eq!(result[0].id, first.id);
    assert!(result[1].line > todos[1].line);
    assert!(
        result
            .iter()
            .all(|t| t.metadata.priority == "p1" && t.source_revision != todos[0].source_revision)
    );
    assert_eq!(result, f.list());
}

#[test]
fn unchanged_stale_dashboard_source_wins_even_when_deleted() {
    let f = Fixture::new("- [ ] One\n");
    f.sync().unwrap();
    fs::write(
        &f.note,
        fs::read_to_string(&f.note)
            .unwrap()
            .replace("One", "Retitled"),
    )
    .unwrap();
    let source = fs::read(&f.note).unwrap();
    f.sync().unwrap();
    assert_eq!(fs::read(&f.note).unwrap(), source);
    assert!(
        fs::read_to_string(&f.dashboard)
            .unwrap()
            .contains("Retitled")
    );
    fs::remove_file(&f.note).unwrap();
    f.sync().unwrap();
    assert!(
        !fs::read_to_string(&f.dashboard)
            .unwrap()
            .contains("Retitled")
    );
}

#[test]
fn changed_stale_dashboard_has_no_partial_writes() {
    let f = Fixture::new("- [ ] One\n- [ ] Two\n");
    f.sync().unwrap();
    f.edit_dashboard();
    fs::write(
        &f.note,
        fs::read_to_string(&f.note).unwrap() + "new prose\n",
    )
    .unwrap();
    let before = f.bytes();
    assert!(f.sync().is_err());
    assert_eq!(f.bytes(), before);
}

#[test]
fn identical_texts_are_not_swapped_across_areas() {
    let f = Fixture::new("- [ ] Same\n  - area: Code\n- [ ] Same\n  - area: Reading\n");
    f.sync().unwrap();
    let content = fs::read_to_string(&f.dashboard)
        .unwrap()
        .replace("## Reading\n- [ ]", "## Reading\n- [x]");
    fs::write(&f.dashboard, content).unwrap();
    f.sync().unwrap();
    let todos = f.list();
    assert!(!todos[0].done);
    assert!(todos[1].done);
}

#[test]
fn malformed_duplicate_legacy_or_visible_edits_preserve_every_byte() {
    for mode in 0..5 {
        let f = Fixture::new("- [ ] One\n");
        f.sync().unwrap();
        let content = fs::read_to_string(&f.dashboard).unwrap();
        let changed = match mode {
            0 => "# Dashboard\n- [x] One [[source]]\n".to_string(),
            1 => content.replace("mw-dashboard:v1:", "mw-dashboard:v9:"),
            2 => content.clone() + content.lines().find(|l| l.starts_with("- [")).unwrap() + "\n",
            3 => content.replace("- [ ] One", "- [ ] Changed"),
            _ => content.replace("[[source]]", "[[other]]"),
        };
        fs::write(&f.dashboard, changed).unwrap();
        let before = f.bytes();
        assert!(f.sync().is_err(), "mode {mode}");
        assert_eq!(f.bytes(), before);
        assert!(f.toggle(&f.list()[0]).is_err());
        assert_eq!(f.bytes(), before);
    }
}

#[test]
fn refresh_requires_explicit_discard_and_never_writes_source() {
    let f = Fixture::new("- [ ] One\n");
    f.sync().unwrap();
    f.edit_dashboard();
    let before = f.bytes();
    assert!(project(f.root.path(), &f.dashboard, false, false).is_err());
    assert!(
        f.toggle(&f.list()[0])
            .unwrap_err()
            .to_string()
            .contains("pending")
    );
    assert_eq!(f.bytes(), before);
    refresh_dashboard_discarding_edits(f.root.path(), &f.dashboard).unwrap();
    assert_eq!(fs::read(&f.note).unwrap(), before.0);
    fs::write(&f.dashboard, "legacy").unwrap();
    assert!(f.sync().is_err());
    refresh_dashboard_discarding_edits(f.root.path(), &f.dashboard).unwrap();
    assert_eq!(fs::read(&f.note).unwrap(), before.0);
}

#[test]
fn multi_source_pending_changes_rejected_before_writes() {
    let f = Fixture::new("- [ ] One\n");
    let other = f.root.path().join("other.md");
    fs::write(
        &other,
        fs::read_to_string(&f.note)
            .unwrap()
            .replace("id: source", "id: other"),
    )
    .unwrap();
    f.sync().unwrap();
    fs::write(
        &f.dashboard,
        fs::read_to_string(&f.dashboard)
            .unwrap()
            .replace("- [ ]", "- [x]"),
    )
    .unwrap();
    let before = f.bytes();
    let other_before = fs::read(&other).unwrap();
    assert!(
        f.sync()
            .unwrap_err()
            .to_string()
            .contains("multiple source")
    );
    assert_eq!(f.bytes(), before);
    assert_eq!(fs::read(&other).unwrap(), other_before);
}

#[test]
fn exact_bytes_crlf_and_no_final_newline_are_preserved() {
    let f = Fixture::new("- [ ] One\n  arbitrary prose\n- [ ] Two");
    let original = fs::read_to_string(&f.note).unwrap().replace('\n', "\r\n");
    fs::write(&f.note, &original).unwrap();
    let todo = f.list().remove(0);
    assert_eq!(todo.source_revision, revision(original.as_bytes()));
    f.toggle(&todo).unwrap();
    assert_eq!(
        fs::read_to_string(&f.note).unwrap(),
        original.replacen("[ ]", "[x]", 1)
    );
    let todo = f.list().remove(0);
    update_task_index_todos(
        f.root.path(),
        &f.dashboard,
        TodoUpdateParams {
            ids: vec![todo.id],
            expected_revision: todo.source_revision,
            title: Some("Renamed".into()),
            ..Default::default()
        },
    )
    .unwrap();
    assert_eq!(
        fs::read_to_string(&f.note).unwrap(),
        original.replacen("[ ] One", "[x] Renamed", 1)
    );
}

#[test]
fn unsafe_options_and_mixed_metadata_prose_are_rejected_losslessly() {
    let f = Fixture::new("- [ ] One\n  - p1 remember important context\n");
    for title in ["bad\nline", "bad\rline", "bad\0line"] {
        let t = &f.list()[0];
        let before = f.bytes();
        assert!(
            update_task_index_todos(
                f.root.path(),
                &f.dashboard,
                TodoUpdateParams {
                    ids: vec![t.id.clone()],
                    expected_revision: t.source_revision.clone(),
                    title: Some(title.into()),
                    ..Default::default()
                }
            )
            .is_err()
        );
        assert_eq!(f.bytes(), before);
    }
    let t = &f.list()[0];
    let before = f.bytes();
    assert!(
        update_task_index_todos(
            f.root.path(),
            &f.dashboard,
            TodoUpdateParams {
                ids: vec![t.id.clone()],
                expected_revision: t.source_revision.clone(),
                priority: Some("p2".into()),
                ..Default::default()
            }
        )
        .is_err()
    );
    assert_eq!(f.bytes(), before);
}

#[test]
fn cooperative_lock_is_not_removed_or_stolen() {
    let f = Fixture::new("- [ ] One");
    let lock = Lock::acquire(f.root.path()).unwrap();
    let content = fs::read(&lock.0).unwrap();
    assert!(
        f.toggle(&f.list()[0])
            .unwrap_err()
            .to_string()
            .contains("PID")
    );
    assert_eq!(fs::read(&lock.0).unwrap(), content);
    drop(lock);
    f.toggle(&f.list()[0]).unwrap();
}

#[test]
fn races_before_commit_preserve_external_changes_and_projection_failure_reports_commit() {
    for stage in ["after-scan", "before-commit", "source-committed"] {
        for change_source in [true, false] {
            if stage == "source-committed" && change_source {
                continue;
            }
            let f = Fixture::new("- [ ] One\n");
            f.sync().unwrap();
            let todo = f.list().remove(0);
            let path = if change_source {
                f.note.clone()
            } else {
                f.dashboard.clone()
            };
            let original = fs::read(&path).unwrap();
            let wanted = stage.to_string();
            HOOK.with(|h| {
                *h.borrow_mut() = Some(Box::new(move |s| {
                    if s == wanted {
                        fs::write(&path, [original.as_slice(), b"external edit"].concat()).unwrap();
                    }
                }))
            });
            let result = f.toggle(&todo);
            HOOK.with(|h| *h.borrow_mut() = None);
            let err = result.unwrap_err().to_string();
            if stage == "source-committed" {
                assert!(
                    err.contains("source committed; dashboard projection NOT updated"),
                    "{err}"
                );
                assert!(fs::read_to_string(&f.note).unwrap().contains("[x]"));
            } else {
                assert!(!fs::read_to_string(&f.note).unwrap().contains("[x]"));
            }
            let path = if change_source { &f.note } else { &f.dashboard };
            assert!(fs::read_to_string(path).unwrap().ends_with("external edit"));
        }
    }
}

#[cfg(unix)]
#[test]
fn symlinks_refused_and_mode_preserved() {
    use std::os::unix::fs::{PermissionsExt, symlink};
    let f = Fixture::new("- [ ] One");
    fs::set_permissions(&f.note, fs::Permissions::from_mode(0o640)).unwrap();
    f.toggle(&f.list()[0]).unwrap();
    assert_eq!(
        fs::metadata(&f.note).unwrap().permissions().mode() & 0o777,
        0o640
    );
    let todo = f.list().remove(0);
    let target = f.root.path().join("target");
    fs::rename(&f.dashboard, &target).unwrap();
    symlink(&target, &f.dashboard).unwrap();
    assert!(f.toggle(&todo).is_err());
    fs::remove_file(&f.dashboard).unwrap();
    fs::rename(&f.note, &target).unwrap();
    symlink(&target, &f.note).unwrap();
    assert!(list_active_task_index_todos(f.root.path()).is_err());
}

#[test]
fn marked_archive_refusal_preserves_full_block_and_dashboard() {
    let f = Fixture::new(
        "- [x] One\n  <!-- mw-task:v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->\n  arbitrary child\n",
    );
    f.sync().unwrap();
    let before = f.bytes();
    assert!(archive_completed_to_life_log(f.root.path()).is_err());
    assert_eq!(f.bytes(), before);
}

#[test]
fn metadata_clear_title_and_toggle_preserve_marker_parent_whitespace_and_children() {
    let f = Fixture::new("\t- [ ] One\n\t  - p1 e:s\n\t  arbitrary child\n");
    let marked = f.identify(&f.list()[0]);
    let marker = format!(
        "\t  <!-- mw-task:v1:{} -->",
        marked.stable_id.as_ref().unwrap()
    );
    let result = update_task_index_todos(
        f.root.path(),
        &f.dashboard,
        TodoUpdateParams {
            ids: vec![marked.id.clone()],
            expected_revision: marked.source_revision,
            title: Some("Renamed".into()),
            clear: vec!["priority".into(), "energy".into()],
            ..Default::default()
        },
    )
    .unwrap();
    let current = &result[0];
    assert_eq!(current.id, marked.id);
    let content = fs::read_to_string(&f.note).unwrap();
    assert!(content.contains("\t- [ ] Renamed\n"));
    assert!(content.contains(&marker));
    assert!(content.contains("\t  arbitrary child\n"));
    assert!(!content.contains("p1 e:s"));
    let toggled = f.toggle(current).unwrap().0;
    assert_eq!(toggled.id, marked.id);
    assert!(fs::read_to_string(&f.note).unwrap().contains(&marker));
}

#[test]
fn malformed_baseline_shape_and_unknown_fields_are_not_ignored() {
    for mode in 0..6 {
        let f = Fixture::new("- [ ] One\n");
        f.sync().unwrap();
        let snapshot = Snapshot::read(&f.dashboard).unwrap();
        let baseline = &dashboard_rows(&snapshot, false).unwrap()[0].0;
        let old = encode(&serde_json::to_vec(baseline).unwrap());
        let mut json = serde_json::to_value(baseline).unwrap();
        match mode {
            0 => {
                json["taskId"] = "unsupported".into();
            }
            1 => {
                json["sourceRevision"] = "sha256:BAD".into();
            }
            2 => {
                json["sourcePath"] = "../escape.md".into();
            }
            3 => {
                json["done"] = "true".into();
            }
            4 => {
                json["unexpected"] = true.into();
            }
            _ => {
                json.as_object_mut().unwrap().remove("text");
            }
        }
        let replacement = encode(&serde_json::to_vec(&json).unwrap());
        fs::write(
            &f.dashboard,
            fs::read_to_string(&f.dashboard)
                .unwrap()
                .replace(&old, &replacement),
        )
        .unwrap();
        let before = f.bytes();
        assert!(f.sync().is_err(), "mode {mode}");
        assert_eq!(f.bytes(), before);
    }
}

#[test]
fn dashboard_preparation_failure_and_source_alias_refuse_before_source_commit() {
    let f = Fixture::new("- [ ] One\n");
    let todo = f.list().remove(0);
    let before = fs::read(&f.note).unwrap();
    let missing = f.root.path().join("missing/dashboard.md");
    assert!(
        toggle_task_index_todo(f.root.path(), &missing, &todo.id, &todo.source_revision).is_err()
    );
    assert_eq!(fs::read(&f.note).unwrap(), before);
    assert!(
        project(f.root.path(), &f.note, false, true)
            .unwrap_err()
            .to_string()
            .contains("active task source")
    );
    assert_eq!(fs::read(&f.note).unwrap(), before);
}

#[test]
fn crlf_metadata_edits_preserve_unaffected_bytes_and_fenced_children_are_not_rewritten() {
    let f = Fixture::new("- [ ] One\n- [ ] Two");
    fs::write(
        &f.note,
        fs::read_to_string(&f.note).unwrap().replace('\n', "\r\n"),
    )
    .unwrap();
    let todo = f.list().remove(0);
    let before = fs::read_to_string(&f.note).unwrap();
    update_task_index_todos(
        f.root.path(),
        &f.dashboard,
        TodoUpdateParams {
            ids: vec![todo.id],
            expected_revision: todo.source_revision,
            priority: Some("p1".into()),
            ..Default::default()
        },
    )
    .unwrap();
    assert_eq!(
        fs::read_to_string(&f.note).unwrap(),
        before.replace("- [ ] One\r\n", "- [ ] One\r\n  - p1\r\n")
    );
    let f = Fixture::new("- [ ] One\n  ```text\n  p1\n  ```\n");
    let todo = f.list().remove(0);
    let before = f.bytes();
    assert!(
        update_task_index_todos(
            f.root.path(),
            &f.dashboard,
            TodoUpdateParams {
                ids: vec![todo.id],
                expected_revision: todo.source_revision,
                priority: Some("p2".into()),
                ..Default::default()
            }
        )
        .is_err()
    );
    assert_eq!(f.bytes(), before);
}
