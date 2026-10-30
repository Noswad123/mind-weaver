use std::{
    collections::{BTreeMap, BTreeSet},
    fs,
    path::Path,
    time::SystemTime,
};

use regex::Regex;

mod safety;
mod source;
pub(crate) use safety::archive::is_managed_archive_document;
pub use safety::archive::{
    ArchiveInspection, ArchiveItem, ArchivePlan, ArchiveResult, ArchiveTaskContextV1,
    ArchiveTodoMetadataV1, commit_archive, inspect_archive, plan_archive, recover_archive,
};
pub use safety::capture::{TaskCaptureResult, capture_task};
pub use safety::migration::{
    TaskMigrationChange, TaskMigrationPlan, TaskMigrationResult, commit_task_migration,
    plan_task_migration, plan_task_migration_with_legacy_dashboard,
};
pub use safety::{
    identify_task_index_todo, refresh_dashboard_discarding_edits, set_task_done,
    toggle_task_index_todo, update_task_index_todos,
};

const FOCUS_GROUPS: &[&str] = &[
    "Code",
    "Action",
    "Reading",
    "Amusement",
    "Music",
    "Exercise",
    "Love",
];

#[derive(Debug, Clone, Default, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TodoMetadata {
    pub status: String,
    pub todo_section: String,
    pub area: String,
    pub priority: String,
    pub energy: String,
    pub weight_override: String,
    pub due: String,
    pub start: String,
    pub estimate: String,
    pub raw: String,
    pub effective_weight: f64,
    pub default_priority: String,
    pub default_energy: String,
}

#[derive(Debug, Clone, Default, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TaskIndexTodo {
    pub id: String,
    #[serde(default)]
    pub stable_id: Option<String>,
    #[serde(default = "positional_identity")]
    pub identity_kind: String,
    #[serde(default)]
    pub source_revision: String,
    pub source_id: String,
    pub source_path: String,
    pub note_title: String,
    pub task_scope: String,
    pub todo_section: String,
    pub area: String,
    pub text: String,
    pub done: bool,
    pub order: usize,
    pub line: usize,
    pub metadata: TodoMetadata,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct TodoUpdateParams {
    pub ids: Vec<String>,
    pub expected_revision: String,
    pub title: Option<String>,
    pub area: Option<String>,
    pub priority: Option<String>,
    pub energy: Option<String>,
    pub weight: Option<String>,
    pub due: Option<String>,
    pub start: Option<String>,
    pub estimate: Option<String>,
    pub metadata: Option<String>,
    pub clear: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct SyncStats {
    pub scanned_markdown_notes: usize,
    pub active_task_index_notes: usize,
    pub synced_tasks: usize,
    pub tasks_by_area: BTreeMap<String, usize>,
    pub source_writebacks: usize,
    pub source_files_updated: usize,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ArchiveStats {
    pub scanned_markdown_notes: usize,
    pub active_task_index_notes: usize,
    pub archived_tasks: usize,
    pub archived_by_area: BTreeMap<String, usize>,
    pub month_files_updated: usize,
    pub source_files_updated: usize,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
struct ParsedTask {
    text: String,
    done: bool,
    order: usize,
    line: usize,
    meta: String,
    section: String,
}

pub fn archive_completed_to_life_log(notes_dir: impl AsRef<Path>) -> std::io::Result<ArchiveStats> {
    let _ = notes_dir;
    Err(safety::invalid(
        "archive is disabled: lossless block preservation and revision/pending-dashboard safety are not yet supported; no files were changed",
    ))
}

pub fn sync_dashboard_from_task_index_notes(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
) -> std::io::Result<SyncStats> {
    safety::project(notes_dir.as_ref(), dashboard_path.as_ref(), true, false)
}

pub fn refresh_dashboard_from_task_index_notes(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
) -> std::io::Result<SyncStats> {
    safety::project(notes_dir.as_ref(), dashboard_path.as_ref(), false, false)
}

pub fn list_active_task_index_todos(
    notes_dir: impl AsRef<Path>,
) -> std::io::Result<(Vec<TaskIndexTodo>, SyncStats)> {
    let (todos, stats, _) = scan_task_snapshots(notes_dir.as_ref())?;
    Ok((todos, stats))
}

fn positional_identity() -> String {
    "positional".to_string()
}

fn scan_task_snapshots(
    root: &Path,
) -> std::io::Result<(Vec<TaskIndexTodo>, SyncStats, BTreeMap<String, String>)> {
    scan_task_snapshots_with(root, None)
}

fn scan_task_snapshots_with(
    root: &Path,
    replacement: Option<(&str, &str)>,
) -> std::io::Result<(Vec<TaskIndexTodo>, SyncStats, BTreeMap<String, String>)> {
    let replacements = replacement
        .into_iter()
        .map(|(path, content)| (path.to_string(), content.to_string()))
        .collect();
    scan_task_snapshots_with_replacements(root, &replacements)
}

fn scan_task_snapshots_with_replacements(
    root: &Path,
    replacements: &BTreeMap<String, String>,
) -> std::io::Result<(Vec<TaskIndexTodo>, SyncStats, BTreeMap<String, String>)> {
    scan_task_snapshots_for_archive(root, replacements, None)
}

// Only archive inspection/recovery supplies a receipt-bound historical source and revision.
fn scan_task_snapshots_for_archive(
    root: &Path,
    replacements: &BTreeMap<String, String>,
    historical: Option<(&str, &str)>,
) -> std::io::Result<(Vec<TaskIndexTodo>, SyncStats, BTreeMap<String, String>)> {
    let mut stats = SyncStats::default();
    let mut todos = Vec::new();
    let mut snapshots = BTreeMap::new();
    let mut source_ids = BTreeSet::new();
    let mut stable_ids = BTreeSet::new();

    let mut paths: BTreeSet<_> = walk_markdown_files(root)?.into_iter().collect();
    for rel in replacements.keys() {
        if rel.is_empty()
            || !Path::new(rel)
                .components()
                .all(|c| matches!(c, std::path::Component::Normal(_)))
            || Path::new(rel).components().any(|c| c.as_os_str() == ".git")
            || !Path::new(rel)
                .extension()
                .is_some_and(|e| e.eq_ignore_ascii_case("md"))
        {
            return Err(safety::invalid(
                "replacement must be a relative Markdown source path",
            ));
        }
        paths.insert(root.join(rel));
    }
    for path in paths {
        stats.scanned_markdown_notes += 1;
        let rel = path
            .strip_prefix(root)
            .unwrap_or(&path)
            .to_string_lossy()
            .replace('\\', "/");
        let content = if let Some(content) = replacements.get(&rel) {
            content.to_string()
        } else {
            fs::read_to_string(&path)?
        };
        let is_inbox = rel.ends_with("introspection/inbox.md");

        let historical_revision = historical.filter(|(name, _)| *name == rel);
        if let Some((_, expected)) = historical_revision
            && safety::revision(content.as_bytes()) != expected
        {
            return Err(safety::invalid(
                "historical source does not match archive receipt",
            ));
        }
        let historical = historical_revision.is_some();
        let metadata = source::metadata_for_archive(&content, historical)
            .map_err(|e| safety::invalid(format!("{rel}: {e}")))?;
        let has_meta = metadata.is_some();
        let metadata = metadata.unwrap_or_default();
        let meta = &metadata.raw;
        let should_process = (has_meta
            && metadata.domains.iter().any(|d| {
                d.eq_ignore_ascii_case("task")
                    || (historical && d.eq_ignore_ascii_case("task-index"))
            })
            && read_bool(meta, "task_active"))
            || (is_inbox && !has_meta);
        if !should_process {
            continue;
        }
        if has_meta {
            stats.active_task_index_notes += 1;
        }

        let note_area = resolve_area(
            meta.get("task_area")
                .map(String::as_str)
                .unwrap_or_default(),
            "Action",
        );
        let task_scope = meta.get("task_scope").cloned().unwrap_or_default();
        let default_priority = meta
            .get("task_default_priority")
            .cloned()
            .unwrap_or_default();
        let default_energy = meta.get("task_default_energy").cloned().unwrap_or_default();
        let source_id = meta
            .get("id")
            .filter(|s| !s.trim().is_empty())
            .cloned()
            .unwrap_or_else(|| rel.clone());
        let note_title = meta
            .get("title")
            .filter(|s| !s.trim().is_empty())
            .cloned()
            .unwrap_or_else(|| file_stem(&rel));

        if !source_ids.insert(source_id.clone()) {
            return Err(safety::invalid(format!(
                "duplicate active source ID: {source_id} in {rel}"
            )));
        }
        let masked = safety::mask_task_examples(&content)
            .map_err(|e| safety::invalid(format!("{rel}: {e}")))?;
        let parsed = extract_todo_tasks_for_archive(&masked, historical)
            .map_err(|e| safety::invalid(format!("{rel}: {e}")))?;
        let identities = safety::identities(&masked, &parsed)
            .map_err(|e| safety::invalid(format!("{rel}: {e}")))?;
        let revision = safety::revision(content.as_bytes());
        snapshots.insert(rel.clone(), content.clone());
        for task in parsed {
            let stable_id = identities.get(&task.line).cloned();
            if let Some(uid) = &stable_id {
                if !stable_ids.insert(uid.clone()) {
                    return Err(safety::invalid(format!(
                        "duplicate global stable task ID: {uid} in {rel}"
                    )));
                }
            }
            let (area, text) =
                resolve_task_area_and_text_with_metadata(&task.text, &task.meta, &note_area);
            if text.trim().is_empty() {
                continue;
            }
            let section = normalize_todo_workflow_section(&task.section);
            let metadata = build_todo_metadata(
                &text,
                &task.meta,
                &area,
                &section,
                task.done,
                &default_priority,
                &default_energy,
            );
            let todo = TaskIndexTodo {
                id: stable_id
                    .as_ref()
                    .map(|uid| format!("task:{uid}"))
                    .unwrap_or_else(|| format!("{}:{}:{}", source_id, task.line, task.order)),
                identity_kind: if stable_id.is_some() {
                    "stable"
                } else {
                    "positional"
                }
                .to_string(),
                stable_id,
                source_revision: revision.clone(),
                source_id: source_id.clone(),
                source_path: rel.clone(),
                note_title: note_title.clone(),
                task_scope: task_scope.clone(),
                todo_section: section,
                area: area.clone(),
                text,
                done: task.done,
                order: task.order,
                line: task.line,
                metadata,
            };
            stats.synced_tasks += 1;
            *stats.tasks_by_area.entry(area).or_default() += 1;
            todos.push(todo);
        }
    }

    todos.sort_by(|a, b| {
        a.source_path
            .cmp(&b.source_path)
            .then(a.order.cmp(&b.order))
    });
    Ok((todos, stats, snapshots))
}

pub fn get_active_task_index_todo(
    notes_dir: impl AsRef<Path>,
    todo_id: &str,
) -> std::io::Result<TaskIndexTodo> {
    if todo_id.trim().is_empty() {
        return Err(std::io::Error::new(
            std::io::ErrorKind::InvalidInput,
            "todo id is required",
        ));
    }
    let (todos, _) = list_active_task_index_todos(notes_dir)?;
    todos
        .into_iter()
        .find(|todo| todo.id == todo_id)
        .ok_or_else(|| {
            std::io::Error::new(
                std::io::ErrorKind::NotFound,
                format!("todo not found: {todo_id}"),
            )
        })
}

fn walk_markdown_files(root: &Path) -> std::io::Result<Vec<std::path::PathBuf>> {
    let mut out = Vec::new();
    walk_markdown_files_into(root, &mut out)?;
    out.sort();
    Ok(out)
}

fn walk_markdown_files_into(dir: &Path, out: &mut Vec<std::path::PathBuf>) -> std::io::Result<()> {
    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        let path = entry.path();
        let file_type = entry.file_type()?;
        if file_type.is_symlink()
            && path
                .extension()
                .and_then(|e| e.to_str())
                .is_some_and(|e| e.eq_ignore_ascii_case("md"))
        {
            return Err(safety::invalid(format!(
                "refusing symlink markdown source: {}",
                path.display()
            )));
        }
        if file_type.is_dir() {
            if entry.file_name() == ".git" {
                continue;
            }
            walk_markdown_files_into(&path, out)?;
        } else if file_type.is_file()
            && path
                .extension()
                .and_then(|e| e.to_str())
                .is_some_and(|e| e.eq_ignore_ascii_case("md"))
        {
            out.push(path);
        }
    }
    Ok(())
}

pub(crate) fn validate_task_vocabulary(content: &str) -> std::io::Result<String> {
    let masked = safety::mask_task_examples(content)?;
    extract_todo_tasks(&masked)?;
    Ok(masked)
}

fn extract_todo_tasks(content: &str) -> std::io::Result<Vec<ParsedTask>> {
    extract_todo_tasks_for_archive(content, false)
}

fn extract_todo_tasks_for_archive(
    content: &str,
    historical: bool,
) -> std::io::Result<Vec<ParsedTask>> {
    let checkbox = Regex::new(r"^\s*[-*]\s*\[([ xX])\]\s+(.+?)\s*$").unwrap();
    let lines: Vec<&str> = content.split('\n').collect();
    let mut out = Vec::new();
    let mut in_todo = false;
    let mut todo_level = 0usize;
    let mut current_section = "Inbox".to_string();
    let mut order = 0usize;

    let mut i = 0usize;
    while i < lines.len() {
        let line = lines[i];
        let trimmed = line.trim();
        if let Some((level, heading)) =
            parse_heading(trimmed).filter(|_| !source::checkbox_like(line))
        {
            if heading.eq_ignore_ascii_case("Todo") {
                if level != 2 && !(historical && level == 1) {
                    return Err(safety::invalid(format!(
                        "unsupported Todo heading on line {}; use '## Todo' with '### Do Now', '### Inbox', and '### Blocked'; migrate legacy headings before task operations",
                        i + 1
                    )));
                }
                in_todo = true;
                todo_level = level;
                current_section = "Inbox".to_string();
            } else if in_todo && level <= todo_level {
                in_todo = false;
                current_section.clear();
            } else if in_todo && level > todo_level {
                let sections: &[&str] = if historical {
                    &["Next", "Waiting", "Done", "Inbox", "Blocked", "Do Now"]
                } else {
                    &["Do Now", "Inbox", "Blocked"]
                };
                if level != todo_level + 1
                    || !sections
                        .iter()
                        .any(|name| heading.eq_ignore_ascii_case(name))
                {
                    return Err(safety::invalid(format!(
                        "unsupported Todo section '{heading}' on line {}; use '### Do Now', '### Inbox', or '### Blocked'; migrate legacy headings before task operations",
                        i + 1
                    )));
                }
                current_section = if historical {
                    sections
                        .iter()
                        .find(|name| heading.eq_ignore_ascii_case(name))
                        .unwrap()
                        .to_string()
                } else {
                    heading
                };
            }
            i += 1;
            continue;
        }
        if !in_todo {
            i += 1;
            continue;
        }
        let Some(captures) = checkbox.captures(trimmed) else {
            if source::checkbox_like(line) {
                return Err(safety::invalid(format!(
                    "unsupported checkbox syntax on line {}; use flat '- [ ]' task blocks",
                    i + 1
                )));
            }
            i += 1;
            continue;
        };
        let done = captures
            .get(1)
            .is_some_and(|m| m.as_str().eq_ignore_ascii_case("x"));
        let text = captures
            .get(2)
            .map(|m| m.as_str().trim().to_string())
            .unwrap_or_default();
        if text.is_empty() {
            i += 1;
            continue;
        }
        let task_indent = line_indent(line);
        let mut j = i + 1;
        let mut meta_parts = Vec::new();
        while j < lines.len() {
            let next = lines[j];
            let next_trimmed = next.trim();
            if next_trimmed.is_empty() {
                j += 1;
                continue;
            }
            if line_indent(next) <= task_indent {
                break;
            }
            if source::checkbox_like(next) {
                return Err(safety::invalid(format!(
                    "unsupported nested checkbox task on line {}; move child tasks to separate sibling blocks before using task operations",
                    j + 1
                )));
            }
            if parse_heading(next_trimmed).is_some() {
                break;
            }
            if !next_trimmed.contains("mw-task:") {
                meta_parts.push(strip_metadata_bullet_prefix(next_trimmed));
            }
            j += 1;
        }
        out.push(ParsedTask {
            text,
            done,
            order,
            line: i + 1,
            meta: meta_parts.join(" "),
            section: current_section.clone(),
        });
        order += 1;
        i = j;
    }
    Ok(out)
}

fn parse_heading(line: &str) -> Option<(usize, String)> {
    let marker = match line.as_bytes().first().copied()? {
        b'#' => b'#',
        b'*' => b'*',
        _ => return None,
    };
    let level = line.as_bytes().iter().take_while(|b| **b == marker).count();
    if level == 0 || line.as_bytes().get(level).copied() != Some(b' ') {
        return None;
    }
    let heading = line[level + 1..].trim();
    (!heading.is_empty()).then(|| (level, heading.to_string()))
}

fn line_indent(line: &str) -> usize {
    line.chars()
        .take_while(|ch| *ch == ' ' || *ch == '\t')
        .count()
}

fn strip_metadata_bullet_prefix(line: &str) -> String {
    Regex::new(r"^[*+-]\s+")
        .unwrap()
        .replace(line.trim(), "")
        .to_string()
}

fn resolve_task_area_and_text_with_metadata(
    task_text: &str,
    metadata_text: &str,
    note_area: &str,
) -> (String, String) {
    if let Some((area, cleaned, valid)) = extract_inline_area(task_text) {
        if valid {
            return (area, cleaned);
        }
        return (note_area.to_string(), task_text.to_string());
    }
    if let Some(area) = extract_area_from_metadata(metadata_text) {
        return (area, task_text.to_string());
    }
    (note_area.to_string(), task_text.to_string())
}

fn extract_inline_area(task_text: &str) -> Option<(String, String, bool)> {
    let re = Regex::new(r"(?i)(?:^|\s)area:([A-Za-z][A-Za-z-]*)\b").unwrap();
    let m = re.find(task_text)?;
    let raw = re.captures(task_text)?.get(1)?.as_str();
    let area = resolve_area(raw, "");
    if area.is_empty() {
        return Some((String::new(), task_text.to_string(), false));
    }
    let cleaned = format!("{} {}", &task_text[..m.start()], &task_text[m.end()..]);
    Some((
        area,
        cleaned.split_whitespace().collect::<Vec<_>>().join(" "),
        true,
    ))
}

fn extract_area_from_metadata(metadata_text: &str) -> Option<String> {
    let re = Regex::new(r"(?i)\barea:\s*([A-Za-z][A-Za-z-]*)\b").unwrap();
    let area = re.captures(metadata_text)?.get(1)?.as_str();
    let area = resolve_area(area, "");
    (!area.is_empty()).then_some(area)
}

fn resolve_area(raw_area: &str, fallback: &str) -> String {
    let raw = raw_area.trim();
    if raw.is_empty() {
        return fallback.to_string();
    }
    for group in FOCUS_GROUPS {
        if group.eq_ignore_ascii_case(raw) {
            return (*group).to_string();
        }
    }
    if raw.eq_ignore_ascii_case("actions") {
        return "Action".to_string();
    }
    fallback.to_string()
}

fn normalize_todo_workflow_section(raw: &str) -> String {
    match raw.trim().to_ascii_lowercase().as_str() {
        "" | "inbox" => "Inbox".to_string(),
        "do now" => "Do Now".to_string(),
        "blocked" => "Blocked".to_string(),
        _ => raw.trim().to_string(),
    }
}

fn build_todo_metadata(
    task_text: &str,
    metadata_text: &str,
    area: &str,
    todo_section: &str,
    done: bool,
    default_priority: &str,
    default_energy: &str,
) -> TodoMetadata {
    let raw = metadata_text.trim();
    let combined = format!("{} {}", task_text.trim(), raw).trim().to_string();
    let mut status = normalize_todo_workflow_section(todo_section);
    if done {
        status = "Done".to_string();
    }
    let default_priority = normalize_priority_or_default(default_priority, "p3");
    let default_energy = normalize_energy_or_default(default_energy, "medium");
    let priority = extract_priority(&combined).unwrap_or_else(|| default_priority.clone());
    let energy = extract_energy(&combined).unwrap_or_else(|| default_energy.clone());
    let weight_override =
        capture_first(r"(?i)\b(?:w|weight):\s*([0-9]+(?:\.[0-9]+)?)\b", &combined);
    TodoMetadata {
        status,
        todo_section: normalize_todo_workflow_section(todo_section),
        area: area.trim().to_string(),
        priority,
        energy,
        weight_override: weight_override.clone(),
        due: capture_first(r"(?i)\bdue:\s*(\d{4}-\d{2}-\d{2})\b", &combined),
        start: capture_first(r"(?i)\bstart:\s*(\d{4}-\d{2}-\d{2})\b", &combined),
        estimate: capture_first(r"(?i)\b(?:est|estimate):\s*([0-9]+)\b", &combined),
        raw: raw.to_string(),
        effective_weight: derive_todo_weight_with_defaults(
            &combined,
            &default_priority,
            &default_energy,
        ),
        default_priority,
        default_energy,
    }
}

fn derive_todo_weight_with_defaults(
    task_text: &str,
    default_priority: &str,
    default_energy: &str,
) -> f64 {
    if let Ok(weight) =
        capture_first(r"(?i)\b(?:w|weight):\s*([0-9]+(?:\.[0-9]+)?)\b", task_text).parse::<f64>()
    {
        if weight > 0.0 {
            return weight;
        }
    }

    let weight = parse_priority_weight(task_text, default_priority)
        * parse_energy_multiplier(task_text, default_energy)
        * parse_due_date_multiplier(task_text)
        * parse_start_date_multiplier(task_text);
    if weight <= 0.0 { 1.0 } else { weight }
}

fn parse_priority_weight(task_text: &str, default_priority: &str) -> f64 {
    let priority = capture_first(r"(?i)\bp:?([1-5])\b", task_text)
        .or_else_empty(|| default_priority.to_string())
        .and_then(|p| normalize_priority_token(&p));
    priority
        .as_deref()
        .map(priority_weight_for_token)
        .unwrap_or(1.0)
}

fn priority_weight_for_token(priority: &str) -> f64 {
    match priority {
        "p1" => 2.0,
        "p2" => 1.5,
        "p3" => 1.0,
        "p4" => 0.75,
        "p5" => 0.5,
        _ => 1.0,
    }
}

fn parse_energy_multiplier(task_text: &str, default_energy: &str) -> f64 {
    let energy = capture_first(
        r"(?i)\be:(xsm|xs|x-small|small|s|medium|m|large|l|xl|x-large)\b",
        task_text,
    )
    .or_else_empty(|| default_energy.to_string())
    .and_then(|e| normalize_energy_token(&e));
    energy
        .as_deref()
        .map(energy_multiplier_for_token)
        .unwrap_or(1.0)
}

fn energy_multiplier_for_token(energy: &str) -> f64 {
    match energy {
        "x-large" => 1.30,
        "large" => 1.15,
        "small" => 0.85,
        "x-small" => 0.70,
        "medium" => 1.0,
        _ => 1.0,
    }
}

fn parse_due_date_multiplier(task_text: &str) -> f64 {
    let due = capture_first(r"(?i)\bdue:\s*(\d{4}-\d{2}-\d{2})\b", task_text);
    let Some(days) = days_until(&due) else {
        return 1.0;
    };
    match days {
        i64::MIN..=-1 => 1.35,
        0 => 1.25,
        1..=3 => 1.15,
        4..=7 => 1.08,
        8..=14 => 1.03,
        _ => 1.0,
    }
}

fn parse_start_date_multiplier(task_text: &str) -> f64 {
    let start = capture_first(r"(?i)\bstart:\s*(\d{4}-\d{2}-\d{2})\b", task_text);
    let Some(days) = days_until(&start) else {
        return 1.0;
    };
    match days {
        i64::MIN..=0 => 1.0,
        1..=7 => 0.85,
        8..=14 => 0.70,
        _ => 0.55,
    }
}

fn days_until(yyyy_mm_dd: &str) -> Option<i64> {
    let target = days_from_ymd(yyyy_mm_dd)?;
    Some(target - today_days_since_unix_epoch())
}

fn today_days_since_unix_epoch() -> i64 {
    let seconds = SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs() as i64;
    seconds / 86_400
}

fn days_from_ymd(yyyy_mm_dd: &str) -> Option<i64> {
    let mut parts = yyyy_mm_dd.split('-');
    let year = parts.next()?.parse::<i64>().ok()?;
    let month = parts.next()?.parse::<i64>().ok()?;
    let day = parts.next()?.parse::<i64>().ok()?;
    if parts.next().is_some() || !(1..=12).contains(&month) || day < 1 {
        return None;
    }
    let max_day = match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        2 if is_leap_year(year) => 29,
        2 => 28,
        _ => return None,
    };
    if day > max_day {
        return None;
    }
    Some(days_from_civil(year, month, day))
}

fn is_leap_year(year: i64) -> bool {
    (year % 4 == 0 && year % 100 != 0) || year % 400 == 0
}

fn days_from_civil(year: i64, month: i64, day: i64) -> i64 {
    let year = year - i64::from(month <= 2);
    let era = if year >= 0 { year } else { year - 399 } / 400;
    let yoe = year - era * 400;
    let month_prime = month + if month > 2 { -3 } else { 9 };
    let doy = (153 * month_prime + 2) / 5 + day - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146_097 + doe - 719_468
}

fn capture_first(pattern: &str, text: &str) -> String {
    Regex::new(pattern)
        .unwrap()
        .captures(text)
        .and_then(|c| c.get(1).map(|m| m.as_str().trim().to_string()))
        .unwrap_or_default()
}

fn extract_priority(text: &str) -> Option<String> {
    capture_first(r"(?i)\bpriority:\s*(p?[1-5])\b", text)
        .or_else_empty(|| capture_first(r"(?i)\bp:?([1-5])\b", text))
        .and_then(|p| normalize_priority_token(&p))
}

fn extract_energy(text: &str) -> Option<String> {
    capture_first(r"(?i)\benergy:\s*([a-zA-Z-]+)\b", text)
        .or_else_empty(|| {
            capture_first(
                r"(?i)\be:(xsm|xs|x-small|small|s|medium|m|large|l|xl|x-large)\b",
                text,
            )
        })
        .and_then(|e| normalize_energy_token(&e))
}

trait EmptyStringOption {
    fn or_else_empty(self, f: impl FnOnce() -> String) -> Option<String>;
}

impl EmptyStringOption for String {
    fn or_else_empty(self, f: impl FnOnce() -> String) -> Option<String> {
        let value = if self.is_empty() { f() } else { self };
        (!value.is_empty()).then_some(value)
    }
}

fn normalize_priority_or_default(value: &str, default: &str) -> String {
    normalize_priority_token(value).unwrap_or_else(|| default.to_string())
}

fn normalize_priority_token(value: &str) -> Option<String> {
    let value = value.trim().to_ascii_lowercase();
    let digits = value.strip_prefix('p').unwrap_or(&value);
    matches!(digits, "1" | "2" | "3" | "4" | "5").then(|| format!("p{digits}"))
}

fn normalize_energy_or_default(value: &str, default: &str) -> String {
    normalize_energy_token(value).unwrap_or_else(|| default.to_string())
}

fn normalize_energy_token(value: &str) -> Option<String> {
    match value.trim().to_ascii_lowercase().as_str() {
        "xsm" | "xs" | "x-small" => Some("x-small".to_string()),
        "small" | "s" => Some("small".to_string()),
        "medium" | "m" => Some("medium".to_string()),
        "large" | "l" => Some("large".to_string()),
        "xl" | "x-large" => Some("x-large".to_string()),
        _ => None,
    }
}

fn read_bool(meta: &BTreeMap<String, String>, key: &str) -> bool {
    let Some(value) = meta.get(key).map(|s| s.trim().to_ascii_lowercase()) else {
        return false;
    };
    matches!(value.as_str(), "true" | "yes" | "on" | "1")
}

fn file_stem(path: &str) -> String {
    let file = path.rsplit('/').next().unwrap_or(path);
    file.rsplit_once('.')
        .map(|(stem, _)| stem)
        .unwrap_or(file)
        .to_string()
}

fn metadata_tokens_from_line(line: &str) -> Option<String> {
    let line = strip_metadata_bullet_prefix(line);
    let mut tokens = Vec::new();
    if let Some(priority) = extract_priority(&line) {
        tokens.push(priority);
    }
    if let Some(energy) = extract_energy(&line) {
        tokens.push(format!("e:{energy}"));
    }
    for (pattern, prefix) in [
        (r"(?i)\b(?:w|weight):\s*([0-9]+(?:\.[0-9]+)?)\b", "w"),
        (r"(?i)\bdue:\s*(\d{4}-\d{2}-\d{2})\b", "due"),
        (r"(?i)\bstart:\s*(\d{4}-\d{2}-\d{2})\b", "start"),
        (r"(?i)\b(?:est|estimate):\s*([0-9]+)\b", "est"),
    ] {
        let value = capture_first(pattern, &line);
        if !value.is_empty() {
            tokens.push(format!("{prefix}:{value}"));
        }
    }
    (!tokens.is_empty()).then(|| tokens.join(" "))
}

fn extract_frontmatter_prefix(content: &str) -> std::io::Result<Option<&str>> {
    let mut lines = content.split_inclusive('\n');
    let Some(first) = lines.next() else {
        return Ok(None);
    };
    let is_fence = |line: &str| matches!(line, "---" | "---\n" | "---\r\n");
    if !is_fence(first) {
        if first.trim_start().starts_with("---") {
            return Err(safety::invalid(
                "malformed frontmatter opening fence; metadata preserved",
            ));
        }
        return Ok(None);
    }
    let mut end = first.len();
    for line in lines {
        let start = end;
        end += line.len();
        if is_fence(line) {
            serde_yaml::from_str::<serde_yaml::Value>(&content[first.len()..start]).map_err(
                |e| safety::invalid(format!("malformed frontmatter: {e}; metadata preserved")),
            )?;
            return Ok(Some(&content[..end]));
        }
    }
    Err(safety::invalid(
        "unterminated frontmatter; metadata preserved even when discarding body edits",
    ))
}

fn update_task_metadata_in_lines(
    mut lines: Vec<String>,
    line_idx: usize,
    todo: &TaskIndexTodo,
    params: &TodoUpdateParams,
) -> std::io::Result<Vec<String>> {
    if !Regex::new(r"^\s*[-*]\s*\[[ xX]\]\s+.+?\s*$")
        .unwrap()
        .is_match(&lines[line_idx])
    {
        return Err(std::io::Error::new(
            std::io::ErrorKind::InvalidData,
            format!("no checkbox task found on line {}", line_idx + 1),
        ));
    }

    let mut todo = todo.clone();
    let cr = if lines[line_idx].ends_with('\r') {
        "\r"
    } else {
        ""
    };
    if let Some(title) = &params.title {
        let title = title.trim();
        if title.is_empty() {
            return Err(std::io::Error::new(
                std::io::ErrorKind::InvalidInput,
                "title cannot be empty",
            ));
        }
        lines[line_idx] = replace_checkbox_task_text(&lines[line_idx], title);
        lines[line_idx].push_str(cr);
        todo.text = title.to_string();
    }

    if [
        &params.area,
        &params.priority,
        &params.energy,
        &params.weight,
        &params.due,
        &params.start,
        &params.estimate,
        &params.metadata,
    ]
    .iter()
    .all(|v| v.is_none())
        && params.clear.is_empty()
    {
        return Ok(lines);
    }

    let task_indent = line_indent(&lines[line_idx]);
    let child_start = line_idx + 1;
    let mut child_end = child_start;
    while child_end < lines.len() {
        let trimmed = lines[child_end].trim();
        if trimmed.is_empty() {
            child_end += 1;
            continue;
        }
        if parse_heading(trimmed).is_some() || line_indent(&lines[child_end]) <= task_indent {
            break;
        }
        child_end += 1;
    }

    let kept_children: Vec<String> = lines[child_start..child_end]
        .iter()
        .filter(|line| !is_task_metadata_line(line.trim()))
        .cloned()
        .collect();
    let meta_line = build_updated_metadata_line(&todo.metadata, params);

    let mut out = Vec::with_capacity(lines.len() + 1);
    out.extend_from_slice(&lines[..child_start]);
    if !meta_line.is_empty() {
        let indent: String = lines[line_idx]
            .chars()
            .take_while(|c| matches!(c, ' ' | '\t'))
            .collect();
        out.push(format!("{indent}  - {meta_line}{cr}"));
    }
    out.extend(kept_children);
    out.extend_from_slice(&lines[child_end..]);
    Ok(out)
}

fn replace_checkbox_task_text(line: &str, title: &str) -> String {
    let indent_len = line.len() - line.trim_start_matches([' ', '\t']).len();
    let indent = &line[..indent_len];
    let trimmed = line.trim();
    let re = Regex::new(r"^([-*]\s*\[[ xX]\]\s+).+?\s*$").unwrap();
    if let Some(captures) = re.captures(trimmed) {
        let prefix = captures.get(1).map(|m| m.as_str()).unwrap_or_default();
        format!("{indent}{prefix}{}", title.trim())
    } else {
        line.to_string()
    }
}

fn build_updated_metadata_line(existing: &TodoMetadata, params: &TodoUpdateParams) -> String {
    if let Some(metadata) = &params.metadata {
        return metadata.trim().to_string();
    }

    let raw = existing.raw.trim();
    let mut area = extract_area_from_metadata(raw).unwrap_or_default();
    let mut priority = extract_priority(raw).unwrap_or_default();
    let mut energy = extract_energy(raw).unwrap_or_default();
    let mut weight = capture_first(r"(?i)\b(?:w|weight):\s*([0-9]+(?:\.[0-9]+)?)\b", raw);
    let mut due = capture_first(r"(?i)\bdue:\s*(\d{4}-\d{2}-\d{2})\b", raw);
    let mut start = capture_first(r"(?i)\bstart:\s*(\d{4}-\d{2}-\d{2})\b", raw);
    let mut estimate = capture_first(r"(?i)\b(?:est|estimate):\s*([0-9]+)\b", raw);

    for clear in &params.clear {
        match clear.trim().to_ascii_lowercase().as_str() {
            "area" => area.clear(),
            "priority" | "p" => priority.clear(),
            "energy" | "e" => energy.clear(),
            "weight" | "w" | "weightoverride" => weight.clear(),
            "due" => due.clear(),
            "start" => start.clear(),
            "estimate" | "est" => estimate.clear(),
            _ => {}
        }
    }
    if let Some(value) = &params.area {
        area = resolve_area(value, value.trim());
    }
    if let Some(value) = &params.priority {
        priority = normalize_priority_or_default(value, "");
    }
    if let Some(value) = &params.energy {
        energy = normalize_energy_or_default(value, "");
    }
    if let Some(value) = &params.weight {
        weight = value.trim().to_string();
    }
    if let Some(value) = &params.due {
        due = value.trim().to_string();
    }
    if let Some(value) = &params.start {
        start = value.trim().to_string();
    }
    if let Some(value) = &params.estimate {
        estimate = value.trim().to_string();
    }

    let mut parts = Vec::new();
    if !area.trim().is_empty() {
        parts.push(format!("area: {}", area.trim()));
    }
    if !priority.trim().is_empty() {
        parts.push(normalize_priority_or_default(&priority, ""));
    }
    if !energy.trim().is_empty() {
        parts.push(format!("e:{}", short_energy_token(&energy)));
    }
    if !weight.trim().is_empty() {
        parts.push(format!("w:{}", weight.trim()));
    }
    if !due.trim().is_empty() {
        parts.push(format!("due:{}", due.trim()));
    }
    if !start.trim().is_empty() {
        parts.push(format!("start:{}", start.trim()));
    }
    if !estimate.trim().is_empty() {
        parts.push(format!("est:{}", estimate.trim()));
    }
    parts.join(" ")
}

fn is_task_metadata_line(trimmed: &str) -> bool {
    if trimmed.is_empty() {
        return false;
    }
    let stripped = strip_metadata_bullet_prefix(trimmed);
    !stripped.trim().is_empty()
        && (extract_area_from_metadata(&stripped).is_some()
            || metadata_tokens_from_line(&stripped).is_some())
}

fn short_energy_token(raw: &str) -> String {
    match normalize_energy_or_default(raw, raw).as_str() {
        "x-small" => "xsm".to_string(),
        "small" => "s".to_string(),
        "medium" => "m".to_string(),
        "large" => "l".to_string(),
        "x-large" => "xl".to_string(),
        _ => raw.trim().to_string(),
    }
}

fn update_checkbox_in_line(line: &str, done: bool) -> String {
    let new_box = if done { "[x]" } else { "[ ]" };
    Regex::new(r"\[[ xX]\]")
        .unwrap()
        .replace(line, new_box)
        .to_string()
}

fn default_dashboard_frontmatter() -> &'static str {
    "---\nid: \"dashboard\"\ntags: []\n---\n"
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn active_legacy_domains_require_migration_and_inactive_sources_stay_inactive() {
        let root = tempfile::tempdir().unwrap();
        let note = root.path().join("tasks.md");
        for domain in ["task-index", "taskIndex", "task"] {
            let content = format!(
                "---\nid: tasks\ndomains: [{domain}]\ntask_active: true\n---\n## Todo\n### Do Now\n- [ ] Work\n"
            );
            fs::write(&note, &content).unwrap();
            if domain == "task" {
                let (todos, _) = list_active_task_index_todos(root.path()).unwrap();
                assert_eq!(todos.len(), 1);
                assert_eq!(todos[0].todo_section, "Do Now");
            } else {
                let error = list_active_task_index_todos(root.path()).unwrap_err();
                assert!(error.to_string().contains("migrate"));
                assert!(error.to_string().contains("tasks.md"));
            }
            assert_eq!(fs::read_to_string(&note).unwrap(), content);
            fs::write(
                &note,
                content.replace("task_active: true", "task_active: false"),
            )
            .unwrap();
            assert!(
                list_active_task_index_todos(root.path())
                    .unwrap()
                    .0
                    .is_empty()
            );
            fs::write(&note, content.replace("task_active: true\n", "")).unwrap();
            assert!(
                list_active_task_index_todos(root.path())
                    .unwrap()
                    .0
                    .is_empty()
            );
        }
    }

    #[test]
    fn todo_sections_reject_legacy_and_unknown_headings_but_ignore_fenced_examples() {
        for heading in ["Next", "Waiting", "Done", "Other"] {
            let content = format!("## Todo\n### {heading}\n- [ ] Work\n");
            let error = validate_task_vocabulary(&content).unwrap_err().to_string();
            assert!(error.contains(heading));
            assert!(error.contains("migrate"));
            assert!(error.contains("Do Now"));
        }
        let content = "## Todo\n### Blocked\n- [ ] Wait\n```markdown\n### Next\n- [ ] Example\n```\n### Inbox\n- [ ] Capture\n~~~markdown\n### Waiting\n~~~\n### Do Now\n- [x] Work\n";
        let masked = validate_task_vocabulary(content).unwrap();
        let tasks = extract_todo_tasks(&masked).unwrap();
        assert_eq!(tasks.len(), 3);
        assert_eq!(tasks[0].section, "Blocked");
        assert_eq!(tasks[1].section, "Inbox");
        assert_eq!(tasks[2].section, "Do Now");
        assert_eq!(tasks[2].line, 14);
    }

    #[test]
    fn lists_active_task_index_todos() {
        let root = tempfile::tempdir().unwrap();
        let note = root.path().join("tasks.md");
        fs::write(
            note,
            r#"---
id: task-note
domains: [task]
task_active: true
task_area: Code
task_default_priority: p2
---

## Todo

### Do Now
- [ ] Ship area:Action p1 due:2026-01-01
  - energy: small
- [x] Done thing
"#,
        )
        .unwrap();

        let (todos, stats) = list_active_task_index_todos(root.path()).unwrap();
        assert_eq!(stats.active_task_index_notes, 1);
        assert_eq!(todos.len(), 2);
        assert_eq!(todos[0].id, "task-note:12:0");
        assert_eq!(todos[0].area, "Action");
        assert_eq!(todos[0].text, "Ship p1 due:2026-01-01");
        assert_eq!(todos[0].metadata.priority, "p1");
        assert_eq!(todos[0].metadata.energy, "small");
        assert_eq!(todos[0].metadata.due, "2026-01-01");
        assert!(todos[1].done);
        assert_eq!(todos[1].metadata.status, "Done");
    }

    #[test]
    fn sync_dashboard_projects_tasks_by_area() {
        let root = tempfile::tempdir().unwrap();
        fs::write(
            root.path().join("hub.md"),
            r#"---
id: productivity-beast
domains: [task]
task_active: true
task_area: Action
---

# Productivity Beast

## Todo
### Do Now
- [ ] 35% Sacred Economics
  - area: Reading
- [ ] Draft architecture review area:Code
  - area: Reading
- [ ] Refill water bottle
"#,
        )
        .unwrap();
        let dashboard = root.path().join("dashboard.md");

        let stats = sync_dashboard_from_task_index_notes(root.path(), &dashboard).unwrap();
        let content = fs::read_to_string(dashboard).unwrap();

        assert_eq!(stats.tasks_by_area.get("Reading"), Some(&1));
        assert_eq!(stats.tasks_by_area.get("Code"), Some(&1));
        assert_eq!(stats.tasks_by_area.get("Action"), Some(&1));
        assert!(content.contains("# Dashboard\n\n## Code"));
        assert!(content.contains("## Reading\n- [ ] 35% Sacred Economics <!-- mw-dashboard:v1:"));
        assert!(content.contains("## Code\n- [ ] Draft architecture review <!-- mw-dashboard:v1:"));
        assert!(content.contains("## Action\n- [ ] Refill water bottle <!-- mw-dashboard:v1:"));
    }

    #[test]
    fn sync_dashboard_only_projects_do_now_section_tasks() {
        let root = tempfile::tempdir().unwrap();
        fs::write(
            root.path().join("hub.md"),
            r#"---
id: productivity-beast
domains: [task]
task_active: true
task_area: Code
---

# Productivity Beast

## Todo
### Inbox
- [ ] Triage idea

### Do Now
- [ ] Ship app fix

### Blocked
- [ ] Wait for review
"#,
        )
        .unwrap();
        let dashboard = root.path().join("dashboard.md");

        let stats = sync_dashboard_from_task_index_notes(root.path(), &dashboard).unwrap();
        let content = fs::read_to_string(dashboard).unwrap();

        assert_eq!(stats.synced_tasks, 1);
        assert!(content.contains("- [ ] Ship app fix <!-- mw-dashboard:v1:"));
        assert!(!content.contains("Triage idea"));
        assert!(!content.contains("Wait for review"));
    }

    #[test]
    fn sync_dashboard_applies_checkbox_writeback() {
        let root = tempfile::tempdir().unwrap();
        let note = root.path().join("hub.md");
        fs::write(
            &note,
            r#"---
id: productivity-beast
domains: [task]
task_active: true
task_area: Action
---

# Productivity Beast

## Todo
### Do Now
- [ ] Ship app fix area:Code
"#,
        )
        .unwrap();
        let dashboard = root.path().join("dashboard.md");
        sync_dashboard_from_task_index_notes(root.path(), &dashboard).unwrap();
        fs::write(
            &dashboard,
            fs::read_to_string(&dashboard)
                .unwrap()
                .replace("- [ ]", "- [x]"),
        )
        .unwrap();

        let stats = sync_dashboard_from_task_index_notes(root.path(), &dashboard).unwrap();
        let source = fs::read_to_string(note).unwrap();
        let dashboard_content = fs::read_to_string(dashboard).unwrap();

        assert_eq!(stats.source_writebacks, 1);
        assert_eq!(stats.source_files_updated, 1);
        assert!(source.contains("- [x] Ship app fix area:Code"));
        assert!(dashboard_content.contains("## Code\n- [x] Ship app fix <!-- mw-dashboard:v1:"));
    }

    #[test]
    fn toggle_task_index_todo_updates_source_and_dashboard() {
        let root = tempfile::tempdir().unwrap();
        let note = root.path().join("hub.md");
        fs::write(
            &note,
            r#"---
id: productivity-beast
domains: [task]
task_active: true
task_area: Action
---

# Productivity Beast

## Todo
### Do Now
- [ ] Ship app fix area:Code
"#,
        )
        .unwrap();
        let dashboard = root.path().join("dashboard.md");
        let (todos, _) = list_active_task_index_todos(root.path()).unwrap();

        let (updated, done) = toggle_task_index_todo(
            root.path(),
            &dashboard,
            &todos[0].id,
            &todos[0].source_revision,
        )
        .unwrap();
        let source = fs::read_to_string(note).unwrap();
        let dashboard_content = fs::read_to_string(dashboard).unwrap();

        assert!(done);
        assert!(updated.done);
        assert!(source.contains("- [x] Ship app fix area:Code"));
        assert!(dashboard_content.contains("## Code\n- [x] Ship app fix <!-- mw-dashboard:v1:"));
    }

    #[test]
    fn update_task_index_todo_updates_metadata_and_dashboard() {
        let root = tempfile::tempdir().unwrap();
        let note = root.path().join("hub.md");
        fs::write(
            &note,
            r#"---
id: productivity-beast
domains: [task]
task_active: true
task_area: Action
---

# Productivity Beast

## Todo
### Do Now
- [ ] Ship app fix
  - p3 e:m
"#,
        )
        .unwrap();
        let dashboard = root.path().join("dashboard.md");
        let (todos, _) = list_active_task_index_todos(root.path()).unwrap();

        let updated = update_task_index_todos(
            root.path(),
            &dashboard,
            TodoUpdateParams {
                ids: vec![todos[0].id.clone()],
                expected_revision: todos[0].source_revision.clone(),
                title: Some("Ship polished app fix".to_string()),
                area: Some("Code".to_string()),
                priority: Some("p1".to_string()),
                due: Some("2026-08-01".to_string()),
                clear: vec!["energy".to_string()],
                ..TodoUpdateParams::default()
            },
        )
        .unwrap();
        let source = fs::read_to_string(note).unwrap();
        let dashboard_content = fs::read_to_string(dashboard).unwrap();

        assert_eq!(updated.len(), 1);
        assert_eq!(updated[0].text, "Ship polished app fix");
        assert_eq!(updated[0].area, "Code");
        assert!(source.contains("- [ ] Ship polished app fix"));
        assert!(source.contains("  - area: Code p1 due:2026-08-01"));
        assert!(!source.contains("e:m"));
        assert!(
            dashboard_content
                .contains("## Code\n- [ ] Ship polished app fix <!-- mw-dashboard:v1:")
        );
    }

    #[test]
    fn archive_refuses_without_modifying_source() {
        let root = tempfile::tempdir().unwrap();
        let note = root.path().join("hub.md");
        fs::write(
            &note,
            r#"---
id: productivity-beast
domains: [task]
task_active: true
task_area: Action
---

# Productivity Beast

## Todo
- [x] Ship app fix area:Code done:2026-06-20
  - p1 e:s est:30
- [ ] Keep working
"#,
        )
        .unwrap();

        let before = fs::read(&note).unwrap();
        assert!(
            archive_completed_to_life_log(root.path())
                .unwrap_err()
                .to_string()
                .contains("disabled")
        );
        assert_eq!(fs::read(&note).unwrap(), before);
        assert!(!root.path().join("introspection").exists());
    }
}
