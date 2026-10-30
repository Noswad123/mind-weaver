//! Source-backed capture. Retry IDs are retained by active tasks and managed archive
//! ledgers, not permanent tombstones: deleting both also deletes the retry history.
use super::*;

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct TaskCaptureResult {
    pub schema_version: String,
    pub task: TaskIndexTodo,
    pub already_captured: bool,
    pub projection_updated: bool,
}

fn nonce() -> io::Result<String> {
    let mut bytes = [0; 16];
    getrandom::fill(&mut bytes).map_err(|e| invalid(format!("random capture identity: {e}")))?;
    Ok(encode(&bytes))
}

fn destination(path: &Path) -> io::Result<Snapshot> {
    if path.as_os_str().is_empty() {
        return Err(invalid("capture requires a nonempty destination path"));
    }
    let name = path
        .file_name()
        .ok_or_else(|| invalid("capture destination needs a final filename"))?;
    let parent = path
        .parent()
        .filter(|p| !p.as_os_str().is_empty())
        .unwrap_or(Path::new("."));
    let path = fs::canonicalize(parent)?.join(name);
    let snapshot = Snapshot::read(&path)?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::MetadataExt;
        if snapshot.bytes.is_some() && fs::metadata(&path)?.nlink() != 1 {
            return Err(invalid(format!(
                "hardlinked capture destination refused: {}",
                path.display()
            )));
        }
    }
    Ok(snapshot)
}

// Return the byte offset immediately after the sole canonical Inbox heading.
fn inbox_offset(content: &str) -> io::Result<usize> {
    let scaffold = || {
        invalid(
            "capture destination needs an active 'domains: [task]', 'task_active: true' source with one '## Todo' and unique '### Do Now', '### Inbox', '### Blocked' headings; scaffold or migrate it explicitly before capture",
        )
    };
    let meta = source::metadata(content)?.ok_or_else(scaffold)?;
    if !read_bool(&meta.raw, "task_active")
        || !meta.domains.iter().any(|d| d.eq_ignore_ascii_case("task"))
    {
        return Err(scaffold());
    }
    let prefix = extract_frontmatter_prefix(content)?.ok_or_else(scaffold)?;
    let mut fence: Option<(u8, usize)> = None;
    for line in content[prefix.len()..].lines() {
        let trimmed = line.trim_start();
        let ch = trimmed.as_bytes().first().copied().unwrap_or_default();
        let count = trimmed.bytes().take_while(|b| *b == ch).count();
        if matches!(ch, b'`' | b'~') && count >= 3 {
            if let Some((marker, size)) = fence {
                if ch == marker && count >= size && trimmed[count..].trim().is_empty() {
                    fence = None;
                }
            } else {
                fence = Some((ch, count));
            }
        }
    }
    if fence.is_some() {
        return Err(invalid(
            "capture destination has an unfinished fenced block",
        ));
    }
    let masked = mask_task_examples(content)?;
    extract_todo_tasks(&masked)?;
    let mut offset = 0;
    let mut todo_count = 0;
    let mut in_todo = false;
    let mut sections = BTreeSet::new();
    let mut inbox = None;
    for line in masked.split_inclusive('\n') {
        offset += line.len();
        if let Some((level, heading)) = parse_heading(line.trim()) {
            if heading.eq_ignore_ascii_case("Todo") {
                if line.trim_end() != "## Todo" {
                    return Err(scaffold());
                }
                todo_count += 1;
                in_todo = true;
            } else if level <= 2 {
                in_todo = false;
            } else if in_todo {
                if level != 3
                    || !["Do Now", "Inbox", "Blocked"].contains(&heading.as_str())
                    || line.trim_end() != format!("### {heading}")
                    || !sections.insert(heading.clone())
                {
                    return Err(scaffold());
                }
                if heading == "Inbox" {
                    // Masking preserves lines but not byte lengths (frontmatter/fences).
                    inbox = Some(masked[..offset].lines().count());
                }
            }
        }
    }
    if todo_count != 1 || sections.len() != 3 {
        return Err(scaffold());
    }
    let heading_lines = inbox.ok_or_else(scaffold)?;
    let offset: usize = content
        .split_inclusive('\n')
        .take(heading_lines)
        .map(str::len)
        .sum();
    if let Some(first) = content[offset..].lines().find(|l| !l.trim().is_empty())
        && line_indent(first) > 0
    {
        return Err(invalid(
            "capture Inbox starts with detached indented content; attach it to an existing task or move it before capture",
        ));
    }
    Ok(offset)
}

/// Capture into the configured Markdown source and refresh the dashboard without DB sync.
/// Supply the same 32-character lowercase hex ID for intentional retries. IDs are
/// not permanent receipts if their source and managed archive history are removed.
pub fn capture_task(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
    inbox_path: impl AsRef<Path>,
    text: &str,
    capture_id: Option<&str>,
) -> io::Result<TaskCaptureResult> {
    if text.trim().is_empty()
        || text.chars().any(char::is_control)
        || ["mw-task", "mw-dashboard", "mw-archive"]
            .iter()
            .any(|s| text.contains(s))
        || source::checkbox_like(text.trim())
    {
        return Err(invalid(
            "capture text must be a nonblank single-line title without controls, nested checkbox syntax, or reserved mw-task/mw-dashboard/mw-archive markers",
        ));
    }
    if capture_id.is_some_and(|id| !valid_hex(id, 32)) {
        return Err(invalid(
            "capture ID must be exactly 32 lowercase hexadecimal characters",
        ));
    }
    let uid = capture_id
        .map(str::to_string)
        .map(Ok)
        .unwrap_or_else(nonce)?;
    let root = fs::canonicalize(notes_dir.as_ref())?;
    if !root.is_dir() {
        return Err(invalid(
            "capture notes root must already exist and be a directory",
        ));
    }
    let source = destination(inbox_path.as_ref())?;
    let dashboard = destination(dashboard_path.as_ref())?;
    let relative = source
        .path
        .strip_prefix(&root)
        .map_err(|_| invalid("capture Inbox must be under the canonical notes root"))?;
    if !relative
        .extension()
        .is_some_and(|e| e.eq_ignore_ascii_case("md"))
    {
        return Err(invalid("capture Inbox must be a Markdown (.md) file"));
    }
    let relative = relative
        .to_str()
        .ok_or_else(|| invalid("capture source path must be UTF-8"))?
        .to_string();
    if Path::new(&relative)
        .components()
        .any(|c| c.as_os_str() == ".git")
    {
        return Err(invalid(
            "capture Inbox must not be inside the excluded .git directory",
        ));
    }
    if source.path == dashboard.path {
        return Err(invalid(
            "capture source and dashboard must be different files",
        ));
    }
    let _lock = Lock::acquire(&root)?;
    source.recheck()?;
    dashboard.recheck()?;
    if pending(&dashboard_rows(&dashboard, false)?) {
        return Err(invalid(
            "pending dashboard checkbox edits; sync or explicitly discard them before capture",
        ));
    }
    let (todos, _, snapshots) = scan_task_snapshots(&root)?;
    checkpoint("after-scan");
    reject_dashboard_source(&root, &dashboard, &snapshots)?;
    source.recheck()?;
    dashboard.recheck()?;
    for (rel, content) in &snapshots {
        source_snapshot(&root, rel, content)?;
    }
    // The archive module owns decoding and validating its frozen ledger grammar.
    if archive::archived_stable_ids(&root)?.contains(&uid) {
        return Err(invalid(format!(
            "capture ID {uid} is already archived; capture never revives archived tasks"
        )));
    }
    let original = std::str::from_utf8(source.bytes.as_deref().unwrap_or_default())
        .map_err(|_| invalid("capture source is not UTF-8"))?;
    let existing = todos
        .iter()
        .find(|t| t.stable_id.as_deref() == Some(uid.as_str()));
    if let Some(task) = existing {
        let literal = snapshots[&task.source_path]
            .lines()
            .nth(task.line - 1)
            .and_then(|line| {
                line.trim_start()
                    .strip_prefix("- [ ] ")
                    .or_else(|| line.trim_start().strip_prefix("- [x] "))
                    .or_else(|| line.trim_start().strip_prefix("- [X] "))
            });
        if task.source_path != relative || literal != Some(text) {
            return Err(invalid(format!(
                "capture ID {uid} already belongs to different literal text or source"
            )));
        }
        // Even a retry must refuse malformed/inactive destination scaffolds.
        inbox_offset(original)?;
        let projection = render(&todos, &dashboard)?;
        let projection_updated = dashboard.bytes.as_deref() != Some(projection.as_slice());
        if projection_updated {
            commit(None, &dashboard, &projection).map_err(|e| {
                invalid(format!(
                    "capture ID {uid}: {e}; inspect source/dashboard before retrying with this ID"
                ))
            })?;
        } else {
            source.recheck()?;
            dashboard.recheck()?;
        }
        return Ok(TaskCaptureResult {
            schema_version: "1".into(),
            task: task.clone(),
            already_captured: true,
            projection_updated,
        });
    }
    let scaffold;
    let content = if original.is_empty() {
        let note_id = nonce()?;
        scaffold = format!(
            "---\nid: {note_id}\ntitle: Task Inbox\ndomains: [task]\ntask_active: true\ntask_scope: personal\ntask_area: Action\n---\n\n## Todo\n### Do Now\n\n### Inbox\n\n### Blocked\n"
        );
        scaffold.as_str()
    } else {
        original
    };
    let offset = inbox_offset(content)?;
    let newline = if content.contains("\r\n") {
        "\r\n"
    } else {
        "\n"
    };
    let mut updated = content[..offset].to_string();
    if !updated.ends_with('\n') {
        updated.push_str(newline);
    }
    updated.push_str(&format!("- [ ] {text}{newline}  <!-- mw-task:v1:{uid} -->"));
    if offset < content.len() || content.ends_with('\n') {
        updated.push_str(newline);
    }
    if offset < content.len() {
        updated.push_str(newline);
    }
    updated.push_str(&content[offset..]);
    let (planned, _, planned_snapshots) =
        scan_task_snapshots_with(&root, Some((&relative, &updated)))?;
    for (rel, before) in &snapshots {
        if *rel != relative && planned_snapshots.get(rel) != Some(before) {
            return Err(invalid(
                "task sources changed during capture preflight; no writes",
            ));
        }
    }
    if planned_snapshots
        .keys()
        .any(|rel| *rel != relative && !snapshots.contains_key(rel))
    {
        return Err(invalid(
            "task sources appeared during capture preflight; no writes",
        ));
    }
    let task = planned
        .iter()
        .find(|t| t.stable_id.as_deref() == Some(uid.as_str()))
        .filter(|t| {
            t.source_path == relative
                && t.todo_section == "Inbox"
                && !t.done
                && t.identity_kind == "stable"
        })
        .cloned()
        .ok_or_else(|| {
            invalid("capture title did not produce a canonical Inbox task; no writes")
        })?;
    let meta = source::metadata(&updated)?.unwrap_or_default();
    let area = resolve_area(
        meta.raw
            .get("task_area")
            .map(String::as_str)
            .unwrap_or_default(),
        "Action",
    );
    if task.text != resolve_task_area_and_text_with_metadata(text.trim(), "", &area).1
        || planned.len() != todos.len() + 1
    {
        return Err(invalid(
            "capture changed task text or structure unexpectedly; no writes",
        ));
    }
    for (before, after) in todos.iter().zip(planned.iter().filter(|t| t.id != task.id)) {
        let mut expected = before.clone();
        if before.source_path == relative {
            expected.source_revision = after.source_revision.clone();
            expected.line = after.line;
            expected.order = after.order;
            if before.stable_id.is_none() {
                expected.id = format!("{}:{}:{}", before.source_id, after.line, after.order);
            }
        }
        if expected != *after {
            return Err(invalid(
                "capture changed an existing task's identity or context; no writes",
            ));
        }
    }
    let projection = render(&planned, &dashboard)?;
    commit(Some((&source, updated.as_bytes())), &dashboard, &projection)
        .map_err(|e| invalid(format!("capture ID {uid}: {e}; inspect source/dashboard before intentionally retrying with this same ID")))?;
    Ok(TaskCaptureResult {
        schema_version: "1".into(),
        task,
        already_captured: false,
        projection_updated: true,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    const A: &str = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
    const B: &str = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
    const SOURCE: &str = "---\nid: source\ntitle: Original\ndomains: [task]\ntask_active: true\ntask_scope: personal\ntask_area: Action\ncustom: [preserve, this]\n---\n## Todo\n### Do Now\n- [ ] Focus\n### Inbox\n\n- [ ] Existing\n  child prose\n  <!-- mw-task:v1:cccccccccccccccccccccccccccccccc -->\n- [ ] Positional\n### Blocked\n- [ ] Wait\n";

    struct Fixture {
        _dir: tempfile::TempDir,
        root: PathBuf,
        source: PathBuf,
        dashboard: PathBuf,
    }
    impl Fixture {
        fn new(content: Option<&str>) -> Self {
            let dir = tempfile::tempdir().unwrap();
            let root = fs::canonicalize(dir.path()).unwrap();
            let source = root.join("configured-inbox.md");
            if let Some(content) = content {
                fs::write(&source, content).unwrap();
            }
            Self {
                source,
                dashboard: root.join("dashboard.md"),
                root,
                _dir: dir,
            }
        }
        fn capture(&self, text: &str, id: Option<&str>) -> io::Result<TaskCaptureResult> {
            capture_task(&self.root, &self.dashboard, &self.source, text, id)
        }
        fn list(&self) -> Vec<TaskIndexTodo> {
            scan_task_snapshots(&self.root).unwrap().0
        }
    }

    #[test]
    fn creates_configured_source_and_serializes_full_result() {
        for initial in [None, Some("")] {
            let f = Fixture::new(initial);
            let r = f.capture("New task", Some(A)).unwrap();
            assert_eq!(r.schema_version, "1");
            assert!(!r.already_captured);
            assert!(r.projection_updated);
            assert_eq!(r.task.id, format!("task:{A}"));
            assert_eq!(r.task.todo_section, "Inbox");
            assert_eq!(r.task.task_scope, "personal");
            assert_eq!(r.task.source_path, "configured-inbox.md");
            assert_eq!(f.list(), vec![r.task.clone()]);
            let json = serde_json::to_value(r).unwrap();
            assert_eq!(json["schemaVersion"], "1");
            assert_eq!(json["alreadyCaptured"], false);
            assert!(
                json["task"]["sourceRevision"]
                    .as_str()
                    .unwrap()
                    .starts_with("sha256:")
            );
        }
    }

    #[test]
    fn preserves_source_children_newlines_and_reparses_positional_ids() {
        for newline in ["\n", "\r\n"] {
            for final_newline in [true, false] {
                let original = if final_newline {
                    SOURCE
                } else {
                    SOURCE.trim_end_matches('\n')
                }
                .replace('\n', newline);
                let f = Fixture::new(Some(&original));
                let before = f.list();
                let r = f.capture("  Literal area:Code  ", Some(A)).unwrap();
                assert_eq!(r.task.text, "Literal");
                let updated = fs::read_to_string(&f.source).unwrap();
                let insertion = format!(
                    "- [ ]   Literal area:Code  {newline}  <!-- mw-task:v1:{A} -->{newline}{newline}"
                );
                assert_eq!(updated.replacen(&insertion, "", 1), original);
                let after = f.list();
                let stable = after
                    .iter()
                    .find(|t| t.stable_id.as_deref() == Some("cccccccccccccccccccccccccccccccc"))
                    .unwrap();
                assert_eq!(stable.metadata, before[1].metadata);
                assert_eq!(stable.id, before[1].id);
                assert_ne!(
                    after.iter().find(|t| t.text == "Positional").unwrap().id,
                    before[2].id
                );
                assert!(fs::read_to_string(&f.dashboard).unwrap().contains("Focus"));
            }
        }
    }

    #[test]
    fn retries_do_not_write_and_compare_literal_not_normalized_text() {
        let f = Fixture::new(Some(SOURCE));
        f.capture("Title area:Code", Some(A)).unwrap();
        let source = fs::read(&f.source).unwrap();
        let dashboard = fs::read(&f.dashboard).unwrap();
        let source_time = fs::metadata(&f.source).unwrap().modified().unwrap();
        let dashboard_time = fs::metadata(&f.dashboard).unwrap().modified().unwrap();
        let r = f.capture("Title area:Code", Some(A)).unwrap();
        assert!(r.already_captured && !r.projection_updated);
        assert_eq!(
            fs::metadata(&f.source).unwrap().modified().unwrap(),
            source_time
        );
        assert_eq!(
            fs::metadata(&f.dashboard).unwrap().modified().unwrap(),
            dashboard_time
        );
        assert!(f.capture("Title", Some(A)).is_err());
        assert!(
            capture_task(
                &f.root,
                &f.dashboard,
                f.root.join("other.md"),
                "Title area:Code",
                Some(A)
            )
            .is_err()
        );
        assert_eq!(fs::read(&f.source).unwrap(), source);
        assert_eq!(fs::read(&f.dashboard).unwrap(), dashboard);
    }

    #[test]
    fn same_title_with_distinct_and_generated_ids_is_not_deduplicated() {
        let f = Fixture::new(None);
        let a = f.capture("Same", Some(A)).unwrap();
        let b = f.capture("Same", Some(B)).unwrap();
        let c = f.capture("Same", None).unwrap();
        assert_ne!(a.task.id, b.task.id);
        assert_ne!(c.task.id, a.task.id);
        assert_ne!(c.task.id, b.task.id);
        assert_eq!(f.list().len(), 3);
    }

    #[test]
    fn invalid_options_fail_before_lock_creation() {
        let f = Fixture::new(None);
        for text in [
            "",
            "  ",
            "one\ntwo",
            "one\rtwo",
            "one\0two",
            "one\ttwo",
            "<!-- mw-task:v1:bad -->",
            "<!-- mw-dashboard -->",
            "<!-- mw-archive-end:v1 -->",
            "- [ ] nested",
        ] {
            assert!(f.capture(text, Some(A)).is_err(), "{text:?}");
        }
        for id in [
            "",
            "Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            "aaa",
            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaag",
        ] {
            assert!(f.capture("Task", Some(id)).is_err());
        }
        assert!(!f.root.join(".mw").exists());
        assert!(!f.source.exists());
        assert!(!f.dashboard.exists());
    }

    #[test]
    fn refuses_inactive_unrelated_legacy_or_ambiguous_scaffolds() {
        for content in [
            SOURCE.replace("task_active: true", "task_active: false"),
            SOURCE.replace("domains: [task]", "domains: [other]"),
            SOURCE.replace("domains: [task]", "domains: [task-index]"),
            SOURCE.replace("### Inbox", "### Waiting"),
            SOURCE.replace("### Inbox", "### Inbox\n### Inbox"),
            SOURCE.replace("## Todo", "## Todo\n## Todo"),
            SOURCE.replace("### Blocked", "## Elsewhere"),
            SOURCE.replace("### Inbox\n", "### Inbox\n  detached prose\n"),
            format!("{SOURCE}\n```unfinished\n"),
            "ordinary unrelated note".into(),
            "---\ndomains: [task\n---\n".into(),
        ] {
            let f = Fixture::new(Some(&content));
            assert!(f.capture("Task", Some(A)).is_err(), "{content}");
            assert_eq!(fs::read_to_string(&f.source).unwrap(), content);
            assert!(!f.dashboard.exists());
        }
    }

    #[test]
    fn pending_and_corrupt_dashboard_block_creation_and_retry() {
        let f = Fixture::new(Some(SOURCE));
        f.capture("Task", Some(A)).unwrap();
        let original = fs::read_to_string(&f.dashboard).unwrap();
        for changed in [original.replacen("- [ ]", "- [x]", 1), "corrupt".into()] {
            fs::write(&f.dashboard, &changed).unwrap();
            let source = fs::read(&f.source).unwrap();
            assert!(f.capture("Task", Some(A)).is_err());
            assert!(f.capture("Other", Some(B)).is_err());
            assert_eq!(fs::read(&f.source).unwrap(), source);
            assert_eq!(fs::read_to_string(&f.dashboard).unwrap(), changed);
        }
    }

    #[test]
    fn paths_require_existing_parents_and_disallow_escape_or_alias() {
        let f = Fixture::new(None);
        let outside = tempfile::tempdir().unwrap();
        for inbox in [
            outside.path().join("outside.md"),
            f.root.join("missing/inbox.md"),
            f.root.join("inbox.txt"),
            f.dashboard.clone(),
        ] {
            assert!(capture_task(&f.root, &f.dashboard, inbox, "Task", Some(A)).is_err());
        }
        for dashboard in [PathBuf::new(), f.root.join("missing/dashboard.md")] {
            assert!(capture_task(&f.root, dashboard, &f.source, "Task", Some(A)).is_err());
        }
        assert!(
            capture_task(
                f.root.join("absent"),
                &f.dashboard,
                &f.source,
                "Task",
                Some(A)
            )
            .is_err()
        );
        assert!(!f.source.exists());
    }

    #[test]
    fn excluded_destinations_cannot_bypass_normal_discovery() {
        let f = Fixture::new(None);
        fs::create_dir(f.root.join(".git")).unwrap();
        let inbox = f.root.join(".git/inbox.md");
        let error = capture_task(&f.root, &f.dashboard, &inbox, "Lost task", Some(A)).unwrap_err();
        assert!(error.to_string().contains("excluded"));
        assert!(!inbox.exists());
        assert!(!f.dashboard.exists());
        assert!(!f.root.join(".mw").exists());
    }

    #[cfg(unix)]
    #[test]
    fn refuses_symlinks_hardlinks_and_parent_alias_escape() {
        use std::os::unix::fs::symlink;
        let f = Fixture::new(None);
        let outside = tempfile::tempdir().unwrap();
        let target = outside.path().join("task.md");
        fs::write(&target, SOURCE).unwrap();
        symlink(&target, &f.source).unwrap();
        assert!(f.capture("Task", Some(A)).is_err());
        fs::remove_file(&f.source).unwrap();
        fs::hard_link(&target, &f.source).unwrap();
        assert!(f.capture("Task", Some(A)).is_err());
        fs::remove_file(&f.source).unwrap();
        symlink(outside.path(), f.root.join("alias")).unwrap();
        assert!(
            capture_task(
                &f.root,
                &f.dashboard,
                f.root.join("alias/new.md"),
                "Task",
                Some(A)
            )
            .is_err()
        );
        symlink(&target, &f.dashboard).unwrap();
        assert!(f.capture("Task", Some(A)).is_err());
    }

    #[test]
    fn duplicate_global_ids_refuse_capture() {
        let f = Fixture::new(Some(SOURCE));
        fs::write(
            f.root.join("other.md"),
            SOURCE.replace("id: source", "id: other"),
        )
        .unwrap();
        assert!(
            f.capture("Task", Some(A))
                .unwrap_err()
                .to_string()
                .contains("duplicate global")
        );
        assert_eq!(fs::read_to_string(&f.source).unwrap(), SOURCE);
    }

    #[test]
    fn concurrent_source_and_dashboard_edits_are_preserved() {
        for stage in ["after-scan", "before-commit"] {
            for edit_source in [true, false] {
                let f = Fixture::new(Some(SOURCE));
                let target = if edit_source {
                    f.source.clone()
                } else {
                    f.dashboard.clone()
                };
                HOOK.with(|h| {
                    *h.borrow_mut() = Some(Box::new(move |at| {
                        if at == stage {
                            fs::write(&target, "external edit").unwrap();
                        }
                    }))
                });
                let result = f.capture("Task", Some(A));
                HOOK.with(|h| *h.borrow_mut() = None);
                assert!(result.is_err());
                let target = if edit_source { &f.source } else { &f.dashboard };
                assert_eq!(fs::read_to_string(target).unwrap(), "external edit");
                if !edit_source {
                    assert_eq!(fs::read_to_string(&f.source).unwrap(), SOURCE);
                }
            }
        }
    }

    #[test]
    fn partial_source_commit_retry_repairs_projection_without_duplicate() {
        let f = Fixture::new(Some(SOURCE));
        let dashboard = f.dashboard.clone();
        HOOK.with(|h| {
            *h.borrow_mut() = Some(Box::new(move |at| {
                if at == "source-committed" {
                    fs::write(&dashboard, "external dashboard edit").unwrap();
                }
            }))
        });
        let error = f.capture("Task", Some(A)).unwrap_err().to_string();
        HOOK.with(|h| *h.borrow_mut() = None);
        assert!(error.contains(A) && error.contains("source committed"));
        let source = fs::read(&f.source).unwrap();
        assert!(f.capture("Task", Some(A)).is_err());
        // Explicitly remove the conflicting projection after inspection.
        fs::remove_file(&f.dashboard).unwrap();
        let retry = f.capture("Task", Some(A)).unwrap();
        assert!(retry.already_captured && retry.projection_updated);
        assert_eq!(fs::read(&f.source).unwrap(), source);
        assert_eq!(
            f.list()
                .iter()
                .filter(|t| t.stable_id.as_deref() == Some(A))
                .count(),
            1
        );
    }

    #[test]
    fn completed_retry_never_reopens_and_archived_id_never_recreates() {
        let f = Fixture::new(None);
        f.capture("Task", Some(A)).unwrap();
        let completed =
            fs::read_to_string(&f.source)
                .unwrap()
                .replacen("- [ ] Task", "- [x] Task", 1);
        fs::write(&f.source, completed).unwrap();
        let retry = f.capture("Task", Some(A)).unwrap();
        assert!(retry.task.done && retry.already_captured);
        let plan = archive::plan_archive(
            &f.root,
            &f.dashboard,
            f.root.join("archive.md"),
            &[retry.task.id],
            &retry.task.source_revision,
        )
        .unwrap();
        archive::commit_archive(&f.root, &f.dashboard, &plan).unwrap();
        let source = fs::read(&f.source).unwrap();
        assert!(
            f.capture("Task", Some(A))
                .unwrap_err()
                .to_string()
                .contains("archived")
        );
        assert_eq!(fs::read(&f.source).unwrap(), source);
    }
}
