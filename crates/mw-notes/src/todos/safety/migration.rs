//! Explicit, read-only preview followed by a confirmed, backed-up whole-root migration.
//! Recheck/rename is not external-editor CAS; multi-file publication is not transactional.
use super::*;
use serde::{Deserialize, Serialize};
use serde_yaml::Value;

fn version<'de, D: serde::Deserializer<'de>>(d: D) -> Result<String, D::Error> {
    let value = String::deserialize(d)?;
    if value != "1" {
        return Err(serde::de::Error::custom(
            "unsupported task migration version",
        ));
    }
    Ok(value)
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct TaskMigrationChange {
    pub source_path: String,
    pub source_revision: String,
    pub replacement: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct TaskMigrationPlan {
    #[serde(deserialize_with = "version")]
    pub schema_version: String,
    pub kind: String,
    pub notes_dir: String,
    pub dashboard_path: String,
    pub dashboard_revision: String,
    #[serde(default)]
    pub bootstrap_legacy_dashboard: bool,
    pub observations: BTreeMap<String, String>,
    pub changes: Vec<TaskMigrationChange>,
    pub blockers: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct TaskMigrationResult {
    #[serde(deserialize_with = "version")]
    pub schema_version: String,
    pub kind: String,
    pub modified_paths: Vec<String>,
    pub backup_path: Option<String>,
    pub projection_updated: bool,
}

fn state(snapshot: &Snapshot) -> String {
    snapshot
        .bytes
        .as_deref()
        .map(revision)
        .unwrap_or_else(|| "absent".into())
}

fn path_string(path: &Path) -> io::Result<String> {
    path.to_str()
        .map(str::to_string)
        .ok_or_else(|| invalid("migration paths must be UTF-8"))
}

fn paths(root: &Path, dashboard: &Path) -> io::Result<(PathBuf, PathBuf)> {
    let root = fs::canonicalize(root)?;
    if !root.is_dir() {
        return Err(invalid("notes root must be a directory"));
    }
    Snapshot::read(dashboard)?;
    let parent = dashboard
        .parent()
        .filter(|p| !p.as_os_str().is_empty())
        .unwrap_or(Path::new("."));
    let dashboard = fs::canonicalize(parent)?.join(
        dashboard
            .file_name()
            .ok_or_else(|| invalid("dashboard filename required"))?,
    );
    Ok((root, dashboard))
}

/// Reads only. Unsupported syntax and unsafe dashboard/archive state are preview blockers.
pub fn plan_task_migration(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
) -> io::Result<TaskMigrationPlan> {
    plan_task_migration_with_legacy_dashboard(notes_dir, dashboard_path, false)
}

/// Allows a reviewed legacy projection upgrade only when every visible row matches a source.
pub fn plan_task_migration_with_legacy_dashboard(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
    allow_legacy_dashboard: bool,
) -> io::Result<TaskMigrationPlan> {
    let (root, dashboard_path) = paths(notes_dir.as_ref(), dashboard_path.as_ref())?;
    let dashboard = Snapshot::read(&dashboard_path)?;
    let mut plan = TaskMigrationPlan {
        schema_version: "1".into(),
        kind: "task-migration-plan".into(),
        notes_dir: path_string(&root)?,
        dashboard_path: path_string(&dashboard_path)?,
        dashboard_revision: state(&dashboard),
        bootstrap_legacy_dashboard: allow_legacy_dashboard,
        observations: BTreeMap::new(),
        changes: Vec::new(),
        blockers: Vec::new(),
    };
    for path in walk_markdown_files(&root)? {
        let rel = path_string(
            path.strip_prefix(&root)
                .map_err(|_| invalid("source outside notes root"))?,
        )?;
        let snapshot = Snapshot::read(&path)?;
        plan.observations.insert(rel.clone(), state(&snapshot));
        let content = std::str::from_utf8(
            snapshot
                .bytes
                .as_deref()
                .ok_or_else(|| invalid("source disappeared during preview"))?,
        )
        .map_err(|_| invalid(format!("{rel}: source is not UTF-8")))?;
        if archive::is_managed_archive_document(content) {
            if let Err(e) = archive::ensure_completed_ledger(&path) {
                plan.blockers.push(format!("{rel}: {e}"));
            }
            continue;
        }
        match transform(content, rel.ends_with("introspection/inbox.md")) {
            Ok(replacement) if replacement != content => {
                if path == dashboard_path {
                    plan.blockers
                        .push("dashboard must not also be a migration source".into());
                }
                plan.changes.push(TaskMigrationChange {
                    source_path: rel,
                    source_revision: state(&snapshot),
                    replacement,
                });
            }
            Err(e) => plan.blockers.push(format!("{rel}: {e}")),
            _ => {}
        }
    }
    let replacements = replacements(&plan);
    if let Err(e) = scan_task_snapshots_with_replacements(&root, &replacements).and_then(
        |(todos, _, sources)| {
            reject_dashboard_source(&root, &dashboard, &sources)?;
            validate_migration_dashboard(&dashboard, &todos, allow_legacy_dashboard)?;
            render(&todos, &dashboard)
        },
    ) {
        plan.blockers
            .push(format!("planned canonical validation: {e}"));
    }
    Ok(plan)
}

fn validate_migration_dashboard(
    dashboard: &Snapshot,
    todos: &[TaskIndexTodo],
    allow_legacy: bool,
) -> io::Result<()> {
    match dashboard_rows(dashboard, false) {
        Ok(rows) => {
            return if pending(&rows) {
                Err(invalid(
                    "pending dashboard checkbox edits; resolve before migration",
                ))
            } else {
                Ok(())
            };
        }
        Err(e) if !allow_legacy => return Err(e),
        Err(_) => {}
    }
    let content = std::str::from_utf8(dashboard.bytes.as_deref().unwrap_or_default())
        .map_err(|_| invalid("dashboard is not UTF-8"))?;
    let prefix = extract_frontmatter_prefix(content)?;
    let body = &content[prefix.map_or(0, str::len)..];
    // Never downgrade any versioned or damaged protocol document to legacy syntax.
    if body.contains("mw-dashboard") {
        return Err(invalid(
            "reserved dashboard markers forbid legacy bootstrap",
        ));
    }
    let checkbox = Regex::new(r"^- \[([ xX])\] (.+) \[\[([^\[\]]+)\]\]$").unwrap();
    let mut title_seen = false;
    let mut groups = 0;
    let mut seen = BTreeSet::new();
    for line in body.lines() {
        if line.trim().is_empty() {
            continue;
        }
        if !title_seen && line == "# Dashboard" {
            title_seen = true;
            continue;
        }
        if title_seen
            && groups < FOCUS_GROUPS.len()
            && line == format!("## {}", FOCUS_GROUPS[groups])
        {
            groups += 1;
            continue;
        }
        let row = checkbox
            .captures(line)
            .filter(|_| title_seen && groups > 0)
            .ok_or_else(|| invalid("unsupported legacy dashboard body or heading order"))?;
        let matches: Vec<_> = todos
            .iter()
            .filter(|t| {
                t.source_id == row[3]
                    && t.text == row[2]
                    && t.todo_section == "Do Now"
                    && t.area == FOCUS_GROUPS[groups - 1]
            })
            .collect();
        if matches.len() != 1 {
            return Err(invalid(
                "unknown or ambiguous legacy dashboard row; unresolved pending text/link edits",
            ));
        }
        let task = matches[0];
        if !seen.insert(task.id.clone()) {
            return Err(invalid("duplicate legacy dashboard row"));
        }
        if task.done != row[1].eq_ignore_ascii_case("x") {
            return Err(invalid(
                "pending legacy dashboard checkbox edits; resolve before migration",
            ));
        }
    }
    if !title_seen || groups != FOCUS_GROUPS.len() {
        return Err(invalid("incomplete legacy dashboard headings"));
    }
    Ok(())
}

fn replacements(plan: &TaskMigrationPlan) -> BTreeMap<String, String> {
    plan.changes
        .iter()
        .map(|c| (c.source_path.clone(), c.replacement.clone()))
        .collect()
}

fn yaml(content: &str) -> io::Result<Option<(usize, usize, Value)>> {
    let Some(prefix) = extract_frontmatter_prefix(content)? else {
        return Ok(None);
    };
    let mut lines = prefix.split_inclusive('\n');
    let start = lines.next().unwrap().len();
    let end = prefix.len() - lines.next_back().unwrap().len();
    let value = serde_yaml::from_str(&prefix[start..end]).map_err(|e| invalid(e.to_string()))?;
    Ok(Some((start, end, value)))
}

fn legacy(value: &str) -> bool {
    value.eq_ignore_ascii_case("task-index") || value.eq_ignore_ascii_case("taskIndex")
}

fn domains(value: &Value) -> io::Result<Vec<String>> {
    match value {
        Value::Null => Ok(Vec::new()),
        Value::String(s) => Ok(crate::parse_list_like_value(s)),
        Value::Sequence(values) => values
            .iter()
            .map(|v| {
                v.as_str()
                    .map(str::to_string)
                    .ok_or_else(|| invalid("non-string domain"))
            })
            .collect(),
        _ => Err(invalid("unsupported domains structure")),
    }
}

fn transform(content: &str, inbox: bool) -> io::Result<String> {
    let mut updated = content.to_string();
    let mut task_source = inbox;
    if let Some((start, end, value)) = yaml(content)? {
        if value.is_null() {
            return Ok(updated);
        }
        let mapping = value
            .as_mapping()
            .ok_or_else(|| invalid("frontmatter must be a mapping"))?;
        let original_domains = domains(mapping.get("domains").unwrap_or(&Value::Null))?;
        task_source = original_domains
            .iter()
            .any(|d| legacy(d) || d.eq_ignore_ascii_case("task"));
        if original_domains.iter().any(|d| legacy(d)) {
            // Only a simple, explicitly located top-level field can be edited losslessly.
            let key = Regex::new(r#"^(?:domains|'domains'|"domains"):[ \t]*(.*)$"#).unwrap();
            let mut offset = start;
            let mut field = None;
            for line in content[start..end].split_inclusive('\n') {
                if key.is_match(line.trim_end_matches(['\r', '\n'])) {
                    field = Some((offset, offset + line.len()));
                } else if let Some((_, stop)) = &mut field {
                    if line.starts_with([' ', '\t', '-'])
                        || line.trim().is_empty()
                        || line.trim_start().starts_with('#')
                    {
                        *stop = offset + line.len();
                    } else {
                        break;
                    }
                }
                offset += line.len();
            }
            let (a, b) = field.ok_or_else(|| {
                invalid("cannot losslessly locate domains; expand complex YAML before migration")
            })?;
            let region = &content[a..b];
            let first = region.lines().next().unwrap();
            let scalar = key.captures(first.trim_end_matches('\r')).unwrap()[1].to_string();
            if scalar.starts_with(['|', '>', '&', '*', '!', '{'])
                || region
                    .lines()
                    .skip(1)
                    .any(|l| !l.trim().is_empty() && !l.trim_start().starts_with(['-', '#']))
            {
                return Err(invalid(
                    "complex domains YAML requires manual review; no reserialization attempted",
                ));
            }
            let token = Regex::new(r"(?i)(?:task-index|taskIndex)").unwrap();
            let mut replacement = String::new();
            for line in region.split_inclusive('\n') {
                // Comments are preserved, including legacy vocabulary in examples.
                let mut quote = None;
                let comment = line
                    .char_indices()
                    .find_map(|(i, c)| {
                        if matches!(c, '\'' | '"') {
                            if quote == Some(c) {
                                quote = None;
                            } else if quote.is_none() {
                                quote = Some(c);
                            }
                        }
                        (c == '#' && quote.is_none()).then_some(i)
                    })
                    .unwrap_or(line.len());
                let text = &line[..comment];
                let mut pos = 0;
                for m in token.find_iter(text) {
                    let boundary = |c: char| !c.is_alphanumeric() && !matches!(c, '_' | '-');
                    if !text[..m.start()].chars().next_back().is_none_or(boundary)
                        || !text[m.end()..].chars().next().is_none_or(boundary)
                    {
                        continue;
                    }
                    replacement.push_str(&text[pos..m.start()]);
                    replacement.push_str("task");
                    pos = m.end();
                }
                replacement.push_str(&text[pos..]);
                replacement.push_str(&line[comment..]);
            }
            updated.replace_range(a..b, &replacement);
            let (_, _, changed) = yaml(&updated)?.unwrap();
            let mut expected = value.clone();
            // Compare the parsed YAML, not just token counts: other fields must be identical.
            let expected_domains = match mapping.get("domains").unwrap() {
                Value::String(s) => Value::String(token.replace_all(s, "task").into_owned()),
                Value::Sequence(values) => Value::Sequence(
                    values
                        .iter()
                        .map(|v| {
                            let s = v.as_str().unwrap();
                            Value::String(if legacy(s) { "task".into() } else { s.into() })
                        })
                        .collect(),
                ),
                _ => return Err(invalid("unsupported domains YAML")),
            };
            expected
                .as_mapping_mut()
                .unwrap()
                .insert(Value::String("domains".into()), expected_domains);
            if changed != expected
                || domains(changed.get("domains").unwrap_or(&Value::Null))?
                    .iter()
                    .any(|d| legacy(d))
            {
                return Err(invalid(
                    "domains edit could not be proven lossless; review YAML manually",
                ));
            }
        }
    }
    if !task_source {
        return Ok(updated);
    }
    let before_masked = mask_task_examples(content)?;
    let before_tasks = extract_todo_tasks_for_archive(&before_masked, true)?;
    let before_ids = identities(&before_masked, &before_tasks)?;
    let masked = mask_task_examples(&updated)?;
    let mut lines: Vec<String> = updated.split('\n').map(str::to_string).collect();
    let mut in_todo = false;
    for (i, line) in masked.split('\n').enumerate() {
        let Some((level, heading)) = parse_heading(line.trim()) else {
            continue;
        };
        if heading.eq_ignore_ascii_case("Todo") {
            if level != 2 {
                return Err(invalid(
                    "ambiguous Todo hierarchy; use ## Todo before migration",
                ));
            }
            in_todo = true;
        } else if in_todo {
            // Match the actual parser's ownership boundary. A level-two Inbox
            // or Next may be unrelated prose, not a malformed Todo child.
            if level <= 2 {
                in_todo = false;
                continue;
            }
            let name = match heading.to_ascii_lowercase().as_str() {
                "next" => Some("Do Now"),
                "waiting" => Some("Blocked"),
                _ => None,
            };
            if let Some(name) = name {
                if !matches!(level, 2 | 3) {
                    return Err(invalid("ambiguous workflow heading depth"));
                }
                let original = &lines[i];
                let trim = original.trim();
                let marker_end = trim.find(' ').unwrap() + 1;
                let heading_start = original.find(trim).unwrap() + marker_end;
                let heading_end = original.trim_end().len();
                let indent = &original[..heading_start - marker_end];
                let suffix = &original[heading_end..];
                lines[i] = format!("{indent}### {name}{suffix}");
            }
        }
    }
    let updated = lines.join("\n");
    let after_masked = mask_task_examples(&updated)?;
    let after_tasks = extract_todo_tasks(&after_masked)?;
    let mut expected = before_tasks;
    for task in &mut expected {
        task.section = match task.section.as_str() {
            "Next" => "Do Now".into(),
            "Waiting" => "Blocked".into(),
            _ => task.section.clone(),
        };
        task.section = normalize_todo_workflow_section(&task.section);
    }
    let mut normalized_after = after_tasks.clone();
    for task in &mut normalized_after {
        task.section = normalize_todo_workflow_section(&task.section);
    }
    if normalized_after != expected || identities(&after_masked, &after_tasks)? != before_ids {
        return Err(invalid(
            "migration changed task inventory, identity, text, completion, metadata or ownership; review source manually",
        ));
    }
    Ok(updated)
}

/// Publishes only the exact confirmed plan. Errors after backups report partial progress.
pub fn commit_task_migration(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
    plan: &TaskMigrationPlan,
) -> io::Result<TaskMigrationResult> {
    // Empty plans are also checked, but must not even create a lock directory.
    let current = plan_task_migration_with_legacy_dashboard(
        &notes_dir,
        &dashboard_path,
        plan.bootstrap_legacy_dashboard,
    )?;
    if current != *plan || !plan.blockers.is_empty() {
        return Err(invalid(
            "migration plan is stale, altered, or blocked; preview again",
        ));
    }
    let dashboard = Snapshot::read(dashboard_path.as_ref())?;
    let upgrade_legacy = plan.bootstrap_legacy_dashboard
        && dashboard.bytes.is_some()
        && dashboard_rows(&dashboard, false).is_err();
    if plan.changes.is_empty() && !upgrade_legacy {
        return Ok(TaskMigrationResult {
            schema_version: "1".into(),
            kind: "task-migration-result".into(),
            modified_paths: Vec::new(),
            backup_path: None,
            projection_updated: false,
        });
    }
    let (root, dashboard_path) = paths(notes_dir.as_ref(), dashboard_path.as_ref())?;
    let _lock = Lock::acquire(&root)?;
    if plan_task_migration_with_legacy_dashboard(
        &root,
        &dashboard_path,
        plan.bootstrap_legacy_dashboard,
    )? != *plan
        || !plan.blockers.is_empty()
    {
        return Err(invalid(
            "migration plan is stale, altered, or blocked; preview again",
        ));
    }
    let dashboard = Snapshot::read(&dashboard_path)?;
    let (todos, _, sources) = scan_task_snapshots_with_replacements(&root, &replacements(plan))?;
    reject_dashboard_source(&root, &dashboard, &sources)?;
    let projection = render(&todos, &dashboard)?;
    let dashboard_temp = dashboard.prepare(&projection)?;
    let mut prepared = Vec::new();
    for change in &plan.changes {
        let snapshot = Snapshot::read(&root.join(&change.source_path))?;
        if state(&snapshot) != change.source_revision {
            return Err(invalid("source changed after migration preflight"));
        }
        let temp = snapshot.prepare(change.replacement.as_bytes())?;
        prepared.push((snapshot, temp));
    }
    let backup = tempfile::Builder::new()
        .prefix("task-migration-")
        .tempdir_in(root.join(".mw"))?
        .keep();
    let backup_path = path_string(&backup)?;
    let mut modified = Vec::new();
    let mut backups_durable = false;
    let publication = (|| -> io::Result<()> {
        let write_backup = |name: &str, bytes: &[u8]| -> io::Result<()> {
            let mut file = fs::OpenOptions::new()
                .write(true)
                .create_new(true)
                .open(backup.join(name))?;
            file.write_all(bytes)?;
            file.sync_all()
        };
        write_backup(
            "plan.json",
            &serde_json::to_vec_pretty(plan).map_err(|e| invalid(e.to_string()))?,
        )?;
        for (i, (snapshot, _)) in prepared.iter().enumerate() {
            write_backup(
                &format!("source-{i}.bak"),
                snapshot
                    .bytes
                    .as_deref()
                    .ok_or_else(|| invalid("source disappeared"))?,
            )?;
        }
        if let Some(bytes) = &dashboard.bytes {
            write_backup("dashboard.bak", bytes)?;
        }
        fs::File::open(&backup)?.sync_all()?;
        fs::File::open(root.join(".mw"))?.sync_all()?;
        backups_durable = true;
        checkpoint("migration-before-commit");
        if plan_task_migration_with_legacy_dashboard(
            &root,
            &dashboard_path,
            plan.bootstrap_legacy_dashboard,
        )? != *plan
        {
            return Err(invalid("migration observations changed before publication"));
        }
        for (snapshot, temp) in prepared {
            dashboard.recheck()?;
            if fs::canonicalize(&snapshot.path)? != snapshot.path {
                return Err(invalid("source path identity changed during migration"));
            }
            snapshot.recheck()?;
            temp.persist(&snapshot.path).map_err(|e| e.error)?;
            modified.push(path_string(&snapshot.path)?);
            fs::File::open(snapshot.path.parent().unwrap())?.sync_all()?;
            checkpoint("migration-source-committed");
        }
        // Recheck all observations, including unmodified canonical sources and ledger evidence.
        let current_paths: BTreeSet<_> = walk_markdown_files(&root)?
            .into_iter()
            .map(|p| path_string(p.strip_prefix(&root).unwrap()))
            .collect::<io::Result<_>>()?;
        if current_paths != plan.observations.keys().cloned().collect() {
            return Err(invalid("Markdown discovery changed during migration"));
        }
        let replacements = replacements(plan);
        for (rel, observed) in &plan.observations {
            let expected = replacements
                .get(rel)
                .map(|s| revision(s.as_bytes()))
                .unwrap_or_else(|| observed.clone());
            if state(&Snapshot::read(&root.join(rel))?) != expected {
                return Err(invalid(format!("source changed during migration: {rel}")));
            }
        }
        dashboard.recheck()?;
        dashboard_temp
            .persist(&dashboard.path)
            .map_err(|e| e.error)?;
        modified.push(path_string(&dashboard.path)?);
        fs::File::open(dashboard.path.parent().unwrap())?.sync_all()?;
        Ok(())
    })();
    if let Err(e) = publication {
        return Err(invalid(format!(
            "migration stopped: {e}; modified paths: {modified:?}; backup directory: {backup_path}; backups durable: {backups_durable}; no automatic rollback; do not blindly retry; inspect files and preview again"
        )));
    }
    Ok(TaskMigrationResult {
        schema_version: "1".into(),
        kind: "task-migration-result".into(),
        modified_paths: modified,
        backup_path: Some(backup_path),
        projection_updated: true,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn note(domain: &str, active: &str) -> String {
        format!(
            "---\nid: tasks\ndomains: [{domain}] # task-index example\ntask_active: {active}\n---\n## Todo\n### Next\n- [ ] Work\n### Waiting\n- [ ] Wait\n```markdown\n### Next\ndomains: [task-index]\n```\n"
        )
    }

    fn legacy_dashboard(rows: &str) -> String {
        let mut body = "# Dashboard\n\n".to_string();
        for area in FOCUS_GROUPS {
            body.push_str(&format!("## {area}\n"));
            if *area == "Action" {
                body.push_str(rows);
            }
            body.push('\n');
        }
        body
    }

    fn legacy_source(canonical: bool) -> String {
        let text = note("task-index", "true")
            .replace("task_active: true", "task_active: true\ntask_area: Action");
        if canonical {
            transform(&text, false).unwrap()
        } else {
            text
        }
    }

    #[test]
    fn legacy_opt_in_upgrades_with_or_without_source_changes_and_preserves_frontmatter() {
        for canonical in [false, true] {
            let root = tempfile::tempdir().unwrap();
            let dashboard = root.path().join("dashboard.md");
            let source = root.path().join("a.md");
            let text = legacy_source(canonical);
            fs::write(&source, &text).unwrap();
            let prefix = "---\r\n# exact custom metadata\r\nid: dashboard\r\ncustom: {quoted: 'value'}\r\n---\r\n";
            let original = format!(
                "{prefix}{}",
                legacy_dashboard("- [ ] Work [[tasks]]\n").replace('\n', "\r\n")
            );
            fs::write(&dashboard, &original).unwrap();
            let default = plan_task_migration(root.path(), &dashboard).unwrap();
            assert!(!default.bootstrap_legacy_dashboard);
            assert!(!default.blockers.is_empty());
            assert!(commit_task_migration(root.path(), &dashboard, &default).is_err());
            let plan =
                plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true).unwrap();
            assert!(plan.bootstrap_legacy_dashboard);
            assert!(plan.blockers.is_empty(), "{:?}", plan.blockers);
            assert_eq!(plan.changes.is_empty(), canonical);
            let encoded = serde_json::to_value(&plan).unwrap();
            assert_eq!(encoded["bootstrapLegacyDashboard"], true);
            let confirmed = serde_json::from_value(encoded).unwrap();
            let result = commit_task_migration(root.path(), &dashboard, &confirmed).unwrap();
            assert!(result.projection_updated);
            assert_eq!(result.modified_paths.len(), if canonical { 1 } else { 2 });
            let backup = Path::new(result.backup_path.as_ref().unwrap());
            assert_eq!(
                fs::read_to_string(backup.join("dashboard.bak")).unwrap(),
                original
            );
            let upgraded = fs::read_to_string(&dashboard).unwrap();
            assert!(upgraded.starts_with(prefix));
            assert!(upgraded.contains(HEADER));
            assert_eq!(
                dashboard_rows(&Snapshot::read(&dashboard).unwrap(), false)
                    .unwrap()
                    .len(),
                1
            );
            let again =
                plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true).unwrap();
            assert!(again.blockers.is_empty());
            assert!(
                !commit_task_migration(root.path(), &dashboard, &again)
                    .unwrap()
                    .projection_updated
            );
        }
    }

    #[test]
    fn legacy_pending_unknown_duplicate_and_ambiguous_rows_never_write() {
        for (rows, duplicate_source_task) in [
            ("- [x] Work [[tasks]]\n", false),
            ("- [X] Work [[tasks]]\n", false),
            ("- [ ] Edited [[tasks]]\n", false),
            ("- [ ] Work [[wrong]]\n", false),
            ("- [ ] Wait [[tasks]]\n", false),
            ("- [ ] Work [[tasks]]\n- [ ] Work [[tasks]]\n", false),
            ("- [ ] Work [[tasks]]\n", true),
        ] {
            let root = tempfile::tempdir().unwrap();
            let dashboard = root.path().join("dashboard.md");
            let source = root.path().join("a.md");
            let mut text = legacy_source(false);
            if duplicate_source_task {
                text = text.replace("- [ ] Work", "- [ ] Work\n- [ ] Work");
            }
            fs::write(&source, &text).unwrap();
            let original = legacy_dashboard(rows);
            fs::write(&dashboard, &original).unwrap();
            let plan =
                plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true).unwrap();
            assert!(!plan.blockers.is_empty(), "{rows}");
            if rows.contains("[x]") || rows.contains("[X]") {
                assert!(plan.blockers.iter().any(|b| b.contains("pending")));
            }
            assert!(commit_task_migration(root.path(), &dashboard, &plan).is_err());
            assert_eq!(fs::read_to_string(&source).unwrap(), text);
            assert_eq!(fs::read_to_string(&dashboard).unwrap(), original);
            assert!(!root.path().join(".mw").exists());
        }
    }

    #[test]
    fn legacy_bootstrap_rejects_body_edits_yaml_and_protocol_downgrades() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        fs::write(root.path().join("a.md"), legacy_source(true)).unwrap();
        project(root.path(), &dashboard, false, false).unwrap();
        let strict = fs::read_to_string(&dashboard).unwrap();
        let legacy = legacy_dashboard("- [ ] Work [[tasks]]\n");
        for original in [
            strict.replacen(HEADER, "<!-- mw-dashboard:v9 -->", 1),
            strict.replace(HEADER, ""),
            strict.replacen("- [ ] Work", "- [x] Work", 1),
            strict.replacen("- [ ] Work", "- [ ] Edited", 1),
            format!("{legacy}<!-- mw-dashboard-baseline:broken -->\n"),
            format!("---\nbroken: [\n---\n{legacy}"),
            format!("{legacy}extra prose\n"),
            legacy.replace("## Code\n\n## Action", "## Action\n\n## Code"),
            legacy.replace("## Action", "## Other"),
            legacy.replace(
                "## Action\n- [ ] Work [[tasks]]",
                "## Action\n\n## Reading\n- [ ] Work [[tasks]]",
            ),
        ] {
            fs::write(&dashboard, &original).unwrap();
            let plan =
                plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true).unwrap();
            assert!(!plan.blockers.is_empty(), "{original}");
            assert!(commit_task_migration(root.path(), &dashboard, &plan).is_err());
            assert_eq!(fs::read_to_string(&dashboard).unwrap(), original);
        }
    }

    #[test]
    fn legacy_stale_dashboard_and_changed_opt_in_are_rejected() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        let source = root.path().join("a.md");
        let text = legacy_source(true);
        fs::write(&source, &text).unwrap();
        let original = legacy_dashboard("- [ ] Work [[tasks]]\n");
        fs::write(&dashboard, &original).unwrap();
        let plan =
            plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true).unwrap();
        let mut altered = plan.clone();
        altered.bootstrap_legacy_dashboard = false;
        assert!(commit_task_migration(root.path(), &dashboard, &altered).is_err());
        let external = format!("{original}\n");
        fs::write(&dashboard, &external).unwrap();
        assert!(commit_task_migration(root.path(), &dashboard, &plan).is_err());
        assert_eq!(fs::read_to_string(&dashboard).unwrap(), external);
        assert_eq!(fs::read_to_string(&source).unwrap(), text);
        assert!(!root.path().join(".mw").exists());
    }

    #[test]
    fn empty_roots_and_legacy_dashboard_have_explicit_clean_semantics() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        let plan =
            plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true).unwrap();
        assert!(plan.blockers.is_empty());
        assert!(
            !commit_task_migration(root.path(), &dashboard, &plan)
                .unwrap()
                .projection_updated
        );
        assert!(!dashboard.exists());
        assert!(!root.path().join(".mw").exists());
        // Old serialized plans default to no opt-in.
        let mut encoded = serde_json::to_value(&plan).unwrap();
        encoded
            .as_object_mut()
            .unwrap()
            .remove("bootstrapLegacyDashboard");
        assert!(
            !serde_json::from_value::<TaskMigrationPlan>(encoded)
                .unwrap()
                .bootstrap_legacy_dashboard
        );
        fs::write(&dashboard, "").unwrap();
        assert!(
            !plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true)
                .unwrap()
                .blockers
                .is_empty()
        );
        fs::write(&dashboard, legacy_dashboard("")).unwrap();
        let plan =
            plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true).unwrap();
        assert!(plan.blockers.is_empty(), "{:?}", plan.blockers);
        assert!(
            commit_task_migration(root.path(), &dashboard, &plan)
                .unwrap()
                .projection_updated
        );
    }

    #[test]
    fn legacy_subset_refreshes_without_losing_source_scan_blockers() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        let source = root.path().join("a.md");
        let text = legacy_source(false).replace("- [ ] Work", "- [x] Work\n- [ ] Newly added");
        fs::write(&source, &text).unwrap();
        fs::write(&dashboard, legacy_dashboard("- [X] Work [[tasks]]\n")).unwrap();
        let broken = root.path().join("broken.md");
        fs::write(&broken, "---\ndomains: &list [task-index]\n---\n").unwrap();
        let blocked =
            plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true).unwrap();
        assert!(blocked.blockers.iter().any(|b| b.contains("broken.md")));
        assert!(commit_task_migration(root.path(), &dashboard, &blocked).is_err());
        assert_eq!(fs::read_to_string(&source).unwrap(), text);
        fs::remove_file(broken).unwrap();
        let plan =
            plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true).unwrap();
        assert!(plan.blockers.is_empty(), "{:?}", plan.blockers);
        let external = dashboard.clone();
        let legacy_bytes = fs::read(&dashboard).unwrap();
        HOOK.with(|h| {
            *h.borrow_mut() = Some(Box::new(move |stage| {
                if stage == "migration-source-committed" {
                    // The reviewed legacy projection stays visible until every source is published.
                    assert_eq!(fs::read(&external).unwrap(), legacy_bytes);
                }
            }))
        });
        let result = commit_task_migration(root.path(), &dashboard, &plan);
        HOOK.with(|h| *h.borrow_mut() = None);
        assert!(result.unwrap().projection_updated);
        let rows = dashboard_rows(&Snapshot::read(&dashboard).unwrap(), false).unwrap();
        assert_eq!(rows.len(), 2);
        assert!(rows.iter().any(|(b, done)| b.text == "Work" && *done));
        assert!(
            rows.iter()
                .any(|(b, done)| b.text == "Newly added" && !*done)
        );
    }

    #[test]
    fn dashboard_only_upgrade_rechecks_after_durable_backup() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        fs::write(root.path().join("a.md"), legacy_source(true)).unwrap();
        fs::write(&dashboard, legacy_dashboard("- [ ] Work [[tasks]]\n")).unwrap();
        let plan =
            plan_task_migration_with_legacy_dashboard(root.path(), &dashboard, true).unwrap();
        assert!(plan.changes.is_empty());
        let external = dashboard.clone();
        HOOK.with(|h| {
            *h.borrow_mut() = Some(Box::new(move |stage| {
                if stage == "migration-before-commit" {
                    fs::write(&external, "external dashboard").unwrap();
                }
            }))
        });
        let result = commit_task_migration(root.path(), &dashboard, &plan);
        HOOK.with(|h| *h.borrow_mut() = None);
        let message = result.unwrap_err().to_string();
        assert!(message.contains("modified paths: []"));
        assert!(message.contains("backups durable: true"));
        assert!(message.contains("do not blindly retry"));
        assert_eq!(
            fs::read_to_string(&dashboard).unwrap(),
            "external dashboard"
        );
    }

    #[test]
    fn pure_preview_roundtrip_and_whole_root_commit() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        let first = note("task-index", "true").replace('\n', "\r\n");
        let second = note("taskIndex", "false").replace("id: tasks", "id: inactive");
        let third = note("task", "true").replace("id: tasks", "id: canonical");
        for (name, text) in [("a.md", &first), ("b.md", &second), ("c.md", &third)] {
            fs::write(root.path().join(name), text).unwrap();
        }
        let plan = plan_task_migration(root.path(), &dashboard).unwrap();
        assert!(plan.blockers.is_empty(), "{:?}", plan.blockers);
        assert_eq!(plan.changes.len(), 3);
        assert!(!root.path().join(".mw").exists());
        assert!(!dashboard.exists());
        assert_eq!(fs::read_to_string(root.path().join("a.md")).unwrap(), first);
        let encoded = serde_json::to_string(&plan).unwrap();
        let confirmed: TaskMigrationPlan = serde_json::from_str(&encoded).unwrap();
        assert_eq!(confirmed, plan);
        let result = commit_task_migration(root.path(), &dashboard, &confirmed).unwrap();
        assert!(result.projection_updated);
        assert_eq!(result.modified_paths.len(), 4);
        let backup = Path::new(result.backup_path.as_ref().unwrap());
        assert_eq!(
            fs::read(backup.join("source-0.bak")).unwrap(),
            first.as_bytes()
        );
        let expected = first
            .replacen("[task-index]", "[task]", 1)
            .replacen("### Next\r\n", "### Do Now\r\n", 1)
            .replacen("### Waiting\r\n", "### Blocked\r\n", 1);
        assert_eq!(
            fs::read_to_string(root.path().join("a.md")).unwrap(),
            expected
        );
        assert!(
            fs::read_to_string(root.path().join("b.md"))
                .unwrap()
                .contains("task_active: false")
        );
        let (todos, _, _) = scan_task_snapshots(root.path()).unwrap();
        assert_eq!(todos.len(), 4);
        assert_eq!(
            todos.iter().filter(|t| t.todo_section == "Do Now").count(),
            2
        );
        assert!(
            plan_task_migration(root.path(), &dashboard)
                .unwrap()
                .changes
                .is_empty()
        );
        assert!(commit_task_migration(root.path(), &dashboard, &plan).is_err());
    }

    #[test]
    fn flow_block_scalar_domains_preserve_sibling_heading_ownership() {
        for field in [
            "domains: [task-index, 'taskIndex', other]",
            "domains:\n  - task-index\n  - taskIndex\n  - other",
            "domains: 'task-index'",
        ] {
            let content = format!(
                "---\n{field}\n---\n## Todo\n### Next\n- [ ] Work\n### Waiting\n- [ ] Wait\n### Inbox\n- [ ] Later\n## Inbox\n- [ ] Documentation example\n## Next\n- [ ] Not a Todo\n## Notes\n### Next\n"
            );
            let changed = transform(&content, false).unwrap();
            assert!(changed.contains("### Do Now\n"));
            assert!(changed.contains("### Blocked\n"));
            assert!(changed.contains("### Inbox\n"));
            assert!(
                changed
                    .contains("## Inbox\n- [ ] Documentation example\n## Next\n- [ ] Not a Todo\n")
            );
            assert_eq!(
                extract_todo_tasks(&mask_task_examples(&changed).unwrap())
                    .unwrap()
                    .len(),
                3
            );
            assert!(changed.ends_with("## Notes\n### Next\n"));
            let (_, _, meta) = yaml(&changed).unwrap().unwrap();
            assert!(
                !domains(meta.get("domains").unwrap())
                    .unwrap()
                    .iter()
                    .any(|d| legacy(d))
            );
        }
    }

    #[test]
    fn complex_yaml_and_ambiguous_headings_block_without_mutation() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        for text in [
            "---\ndomains: &list [task-index]\ntask_active: true\n---\n## Todo\n### Next\n",
            "---\ndomains: >-\n  task-index\ntask_active: true\n---\n## Todo\n### Next\n",
            "---\ndomains: [task]\ntask_active: true\n---\n## Todo\n#### Next\n",
        ] {
            fs::write(root.path().join("a.md"), text).unwrap();
            let plan = plan_task_migration(root.path(), &dashboard).unwrap();
            assert!(!plan.blockers.is_empty());
            assert!(commit_task_migration(root.path(), &dashboard, &plan).is_err());
            assert_eq!(fs::read_to_string(root.path().join("a.md")).unwrap(), text);
            assert!(!dashboard.exists());
        }
    }

    #[test]
    fn stale_and_tampered_observations_are_rejected() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        let source = root.path().join("a.md");
        let text = note("task-index", "true");
        fs::write(&source, &text).unwrap();
        let plan = plan_task_migration(root.path(), &dashboard).unwrap();
        let mut altered = plan.clone();
        altered.changes[0].replacement.push_str("injected\n");
        assert!(commit_task_migration(root.path(), &dashboard, &altered).is_err());
        fs::write(root.path().join("unrelated.md"), "a new observation").unwrap();
        assert!(commit_task_migration(root.path(), &dashboard, &plan).is_err());
        assert_eq!(fs::read_to_string(&source).unwrap(), text);
        assert!(!dashboard.exists());
        let json = serde_json::to_value(&plan).unwrap();
        let mut future = json.clone();
        future["schemaVersion"] = "2".into();
        assert!(serde_json::from_value::<TaskMigrationPlan>(future).is_err());
        let mut unknown = json;
        unknown["surprise"] = true.into();
        assert!(serde_json::from_value::<TaskMigrationPlan>(unknown).is_err());
    }

    #[test]
    fn empty_commit_does_not_create_lock_or_dashboard() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        let plan = plan_task_migration(root.path(), &dashboard).unwrap();
        assert!(plan.blockers.is_empty());
        let result = commit_task_migration(root.path(), &dashboard, &plan).unwrap();
        assert!(result.modified_paths.is_empty());
        assert!(result.backup_path.is_none());
        assert!(!root.path().join(".mw").exists());
        assert!(!dashboard.exists());
    }

    #[test]
    fn pending_and_corrupt_dashboard_are_preview_blockers() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        fs::write(
            root.path().join("a.md"),
            note("task", "true")
                .replacen("### Next", "### Do Now", 1)
                .replacen("### Waiting", "### Blocked", 1),
        )
        .unwrap();
        project(root.path(), &dashboard, false, false).unwrap();
        let pending =
            fs::read_to_string(&dashboard)
                .unwrap()
                .replacen("- [ ] Work", "- [x] Work", 1);
        fs::write(&dashboard, &pending).unwrap();
        let plan = plan_task_migration(root.path(), &dashboard).unwrap();
        assert!(plan.blockers.iter().any(|b| b.contains("pending")));
        assert!(commit_task_migration(root.path(), &dashboard, &plan).is_err());
        assert_eq!(fs::read_to_string(&dashboard).unwrap(), pending);
        fs::write(&dashboard, "human dashboard").unwrap();
        assert!(
            !plan_task_migration(root.path(), &dashboard)
                .unwrap()
                .blockers
                .is_empty()
        );
    }

    #[test]
    fn historical_archive_is_opaque_and_incomplete_ledger_blocks() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        let historical = include_str!("archive/v1-ledger.md");
        // The frozen fixture is an archiveCopied operation. Derive a completed
        // receipt in memory without changing its frozen source/payload history.
        let receipt = historical
            .lines()
            .find_map(|line| {
                line.strip_prefix("<!-- mw-archive-receipt:v1:")
                    .and_then(|s| s.strip_suffix(" -->"))
            })
            .unwrap();
        let json = String::from_utf8(decode(receipt).unwrap()).unwrap();
        let completed = json.replacen("\"phase\":\"archiveCopied\"", "\"phase\":\"completed\"", 1);
        assert_ne!(json, completed);
        let completed = historical.replacen(receipt, &encode(completed.as_bytes()), 1);
        fs::write(root.path().join("history.md"), &completed).unwrap();
        archive::ensure_completed_ledger(&root.path().join("history.md")).unwrap();
        let plan = plan_task_migration(root.path(), &dashboard).unwrap();
        assert!(plan.blockers.is_empty(), "{:?}", plan.blockers);
        assert!(plan.changes.is_empty());
        let prefix = extract_frontmatter_prefix(historical).unwrap().unwrap();
        let incomplete = format!("{prefix}<!-- mw-archive-ledger:v1 -->\n");
        fs::write(root.path().join("history.md"), &incomplete).unwrap();
        let plan = plan_task_migration(root.path(), &dashboard).unwrap();
        assert!(plan.blockers.iter().any(|b| b.contains("incomplete")));
        assert_eq!(
            fs::read_to_string(root.path().join("history.md")).unwrap(),
            incomplete
        );
    }

    #[test]
    fn virtual_replacement_discovery_includes_nonexistent_source() {
        let root = tempfile::tempdir().unwrap();
        let text = note("task", "true")
            .replacen("### Next", "### Do Now", 1)
            .replacen("### Waiting", "### Blocked", 1);
        let replacements = BTreeMap::from([("planned.md".into(), text)]);
        assert_eq!(
            scan_task_snapshots_with_replacements(root.path(), &replacements)
                .unwrap()
                .0
                .len(),
            2
        );
        assert!(!root.path().join("planned.md").exists());
    }

    #[test]
    fn partial_failure_preserves_external_edits_and_reports_backups() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        let a = root.path().join("a.md");
        let b = root.path().join("b.md");
        fs::write(&a, note("task-index", "true")).unwrap();
        fs::write(
            &b,
            note("taskIndex", "true").replace("id: tasks", "id: other"),
        )
        .unwrap();
        let plan = plan_task_migration(root.path(), &dashboard).unwrap();
        assert!(plan.blockers.is_empty());
        let external = b.clone();
        HOOK.with(|h| {
            *h.borrow_mut() = Some(Box::new(move |stage| {
                if stage == "migration-source-committed" {
                    fs::write(&external, "external editor").unwrap();
                }
            }))
        });
        let result = commit_task_migration(root.path(), &dashboard, &plan);
        HOOK.with(|h| *h.borrow_mut() = None);
        let message = result.unwrap_err().to_string();
        assert!(message.contains("modified paths:"));
        assert!(message.contains("a.md"));
        assert!(message.contains("backup directory:"));
        assert!(message.contains("backups durable: true"));
        assert!(message.contains("do not blindly retry"));
        assert_eq!(fs::read_to_string(&b).unwrap(), "external editor");
        assert!(fs::read_to_string(&a).unwrap().contains("### Do Now"));
        assert!(!dashboard.exists());
    }

    #[test]
    fn dashboard_changed_after_source_publication_is_preserved() {
        let root = tempfile::tempdir().unwrap();
        let dashboard = root.path().join("dashboard.md");
        let source = root.path().join("a.md");
        fs::write(&source, note("task-index", "true")).unwrap();
        let plan = plan_task_migration(root.path(), &dashboard).unwrap();
        let external = dashboard.clone();
        HOOK.with(|h| {
            *h.borrow_mut() = Some(Box::new(move |stage| {
                if stage == "migration-source-committed" {
                    fs::write(&external, "external dashboard").unwrap();
                }
            }))
        });
        let result = commit_task_migration(root.path(), &dashboard, &plan);
        HOOK.with(|h| *h.borrow_mut() = None);
        let message = result.unwrap_err().to_string();
        assert!(message.contains("modified paths:"));
        assert!(message.contains("a.md"));
        assert_eq!(
            fs::read_to_string(&dashboard).unwrap(),
            "external dashboard"
        );
        assert!(fs::read_to_string(&source).unwrap().contains("### Do Now"));
    }
}
