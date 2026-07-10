use std::{collections::BTreeMap, fs, path::Path, time::SystemTime};

use regex::Regex;

use crate::extract_metadata;

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
pub struct SyncStats {
    pub scanned_markdown_notes: usize,
    pub active_task_index_notes: usize,
    pub synced_tasks: usize,
    pub tasks_by_area: BTreeMap<String, usize>,
    pub source_writebacks: usize,
    pub source_files_updated: usize,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
struct SourcedTask {
    area: String,
    text: String,
    done: bool,
    source_path: String,
    source_id: String,
    order: usize,
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

pub fn sync_dashboard_from_task_index_notes(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
) -> std::io::Result<SyncStats> {
    sync_dashboard_from_task_index_notes_with_options(notes_dir, dashboard_path, true)
}

pub fn refresh_dashboard_from_task_index_notes(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
) -> std::io::Result<SyncStats> {
    sync_dashboard_from_task_index_notes_with_options(notes_dir, dashboard_path, false)
}

fn sync_dashboard_from_task_index_notes_with_options(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
    apply_dashboard_selections: bool,
) -> std::io::Result<SyncStats> {
    let root = notes_dir.as_ref();
    let dashboard_path = dashboard_path.as_ref();
    let mut stats = SyncStats::default();
    let mut tasks = Vec::new();
    let dashboard_selections = if apply_dashboard_selections {
        read_dashboard_selections(dashboard_path)?
    } else {
        BTreeMap::new()
    };
    let mut selection_cursor: BTreeMap<String, usize> = BTreeMap::new();

    for path in walk_markdown_files(root)? {
        stats.scanned_markdown_notes += 1;
        let content = fs::read_to_string(&path)?;
        let rel = path
            .strip_prefix(root)
            .unwrap_or(&path)
            .to_string_lossy()
            .replace('\\', "/");
        let is_inbox = rel.ends_with("introspection/inbox.md");

        let metadata = extract_metadata(&content);
        let meta = &metadata.raw;
        let has_meta = has_frontmatter(&content);
        let should_process = (has_meta
            && metadata
                .domains
                .iter()
                .any(|d| d.eq_ignore_ascii_case("task-index"))
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
        let source_id = meta
            .get("id")
            .filter(|s| !s.trim().is_empty())
            .cloned()
            .unwrap_or_else(|| rel.clone());
        let mut parsed = extract_todo_tasks(&content);
        if parsed.is_empty() {
            continue;
        }

        let mut lines: Vec<String> = content.split('\n').map(str::to_string).collect();
        let mut note_changed = false;
        for task in &mut parsed {
            let (area, text) =
                resolve_task_area_and_text_with_metadata(&task.text, &task.meta, &note_area);
            if text.trim().is_empty() {
                continue;
            }

            let key = selection_key(&source_id, &text);
            if let Some(picks) = dashboard_selections.get(&key) {
                let idx = selection_cursor.entry(key).or_default();
                if *idx < picks.len() {
                    let desired_done = picks[*idx];
                    *idx += 1;
                    if task.done != desired_done {
                        let line_idx = task.line.saturating_sub(1);
                        if line_idx < lines.len() {
                            let updated = update_checkbox_in_line(&lines[line_idx], desired_done);
                            if updated != lines[line_idx] {
                                lines[line_idx] = updated;
                                task.done = desired_done;
                                note_changed = true;
                                stats.source_writebacks += 1;
                            }
                        }
                    }
                }
            }

            tasks.push(SourcedTask {
                area,
                text,
                done: task.done,
                source_path: rel.clone(),
                source_id: source_id.clone(),
                order: task.order,
            });
        }

        if note_changed {
            fs::write(&path, lines.join("\n"))?;
            stats.source_files_updated += 1;
        }
    }

    tasks.sort_by(|a, b| {
        a.source_path
            .cmp(&b.source_path)
            .then(a.order.cmp(&b.order))
    });

    let mut grouped: BTreeMap<String, Vec<SourcedTask>> = BTreeMap::new();
    for group in FOCUS_GROUPS {
        grouped.insert((*group).to_string(), Vec::new());
    }
    for task in tasks {
        stats.synced_tasks += 1;
        *stats.tasks_by_area.entry(task.area.clone()).or_default() += 1;
        grouped.entry(task.area.clone()).or_default().push(task);
    }

    write_dashboard_projection(dashboard_path, &grouped)?;
    Ok(stats)
}

pub fn list_active_task_index_todos(
    notes_dir: impl AsRef<Path>,
) -> std::io::Result<(Vec<TaskIndexTodo>, SyncStats)> {
    let root = notes_dir.as_ref();
    let mut stats = SyncStats::default();
    let mut todos = Vec::new();

    for path in walk_markdown_files(root)? {
        stats.scanned_markdown_notes += 1;
        let content = fs::read_to_string(&path)?;
        let rel = path
            .strip_prefix(root)
            .unwrap_or(&path)
            .to_string_lossy()
            .replace('\\', "/");
        let is_inbox = rel.ends_with("introspection/inbox.md");

        let metadata = extract_metadata(&content);
        let meta = &metadata.raw;
        let has_meta = has_frontmatter(&content);
        let should_process = (has_meta
            && metadata
                .domains
                .iter()
                .any(|d| d.eq_ignore_ascii_case("task-index"))
            && read_bool(&meta, "task_active"))
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

        for task in extract_todo_tasks(&content) {
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
                id: format!("{}:{}:{}", source_id, task.line, task.order),
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
    Ok((todos, stats))
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

fn extract_todo_tasks(content: &str) -> Vec<ParsedTask> {
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
        if let Some((level, heading)) = parse_heading(trimmed) {
            if heading.eq_ignore_ascii_case("Todo") {
                in_todo = true;
                todo_level = level;
                current_section = "Inbox".to_string();
            } else if in_todo && level <= todo_level {
                in_todo = false;
                current_section.clear();
            } else if in_todo && level > todo_level {
                current_section = heading;
            }
            i += 1;
            continue;
        }
        if !in_todo {
            i += 1;
            continue;
        }
        let Some(captures) = checkbox.captures(trimmed) else {
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
            if parse_heading(next_trimmed).is_some() || line_indent(next) <= task_indent {
                break;
            }
            meta_parts.push(strip_metadata_bullet_prefix(next_trimmed));
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
        i += 1;
    }
    out
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
        "next" => "Next".to_string(),
        "waiting" => "Waiting".to_string(),
        "blocked" => "Blocked".to_string(),
        "done" => "Done".to_string(),
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

fn has_frontmatter(content: &str) -> bool {
    content
        .lines()
        .next()
        .is_some_and(|line| line.trim() == "---")
}

fn file_stem(path: &str) -> String {
    let file = path.rsplit('/').next().unwrap_or(path);
    file.rsplit_once('.')
        .map(|(stem, _)| stem)
        .unwrap_or(file)
        .to_string()
}

fn write_dashboard_projection(
    dashboard_path: &Path,
    grouped: &BTreeMap<String, Vec<SourcedTask>>,
) -> std::io::Result<()> {
    let mut prefix = default_dashboard_frontmatter().to_string();
    if let Ok(content) = fs::read_to_string(dashboard_path) {
        if let Some(frontmatter) = extract_frontmatter_prefix(&content) {
            prefix = frontmatter;
        }
    }

    if let Some(parent) = dashboard_path.parent() {
        if !parent.as_os_str().is_empty() {
            fs::create_dir_all(parent)?;
        }
    }

    let mut out = String::new();
    out.push_str(&prefix);
    if !prefix.ends_with('\n') {
        out.push('\n');
    }
    out.push_str("# Dashboard\n\n");
    for group in FOCUS_GROUPS {
        out.push_str("## ");
        out.push_str(group);
        out.push('\n');
        if let Some(tasks) = grouped.get(*group) {
            for task in tasks {
                let box_text = if task.done { "[x]" } else { "[ ]" };
                let mut text = task.text.trim().to_string();
                if !task.source_id.trim().is_empty() {
                    text.push_str(" [[");
                    text.push_str(task.source_id.trim());
                    text.push_str("]]");
                }
                out.push_str("- ");
                out.push_str(box_text);
                out.push(' ');
                out.push_str(&text);
                out.push('\n');
            }
        }
        out.push('\n');
    }

    fs::write(dashboard_path, out)
}

fn extract_frontmatter_prefix(content: &str) -> Option<String> {
    let mut lines = content.split('\n');
    if lines.next()?.trim() != "---" {
        return None;
    }
    let mut prefix = vec!["---".to_string()];
    for line in lines {
        prefix.push(line.to_string());
        if line.trim() == "---" {
            return Some(format!("{}\n", prefix.join("\n")));
        }
    }
    None
}

fn read_dashboard_selections(
    dashboard_path: &Path,
) -> std::io::Result<BTreeMap<String, Vec<bool>>> {
    let mut out: BTreeMap<String, Vec<bool>> = BTreeMap::new();
    let content = match fs::read_to_string(dashboard_path) {
        Ok(content) => content,
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => return Ok(out),
        Err(err) => return Err(err),
    };
    let checkbox = Regex::new(r"^\s*[-*]\s*\[([ xX])\]\s+(.+?)\s*$").unwrap();
    for line in content.lines() {
        let Some(captures) = checkbox.captures(line.trim()) else {
            continue;
        };
        let task_text = captures
            .get(2)
            .map(|m| m.as_str().trim())
            .unwrap_or_default();
        let Some((text, source_id)) = split_dashboard_task_source(task_text) else {
            continue;
        };
        let done = captures
            .get(1)
            .is_some_and(|m| m.as_str().eq_ignore_ascii_case("x"));
        out.entry(selection_key(&source_id, &text))
            .or_default()
            .push(done);
    }
    Ok(out)
}

fn split_dashboard_task_source(task_text: &str) -> Option<(String, String)> {
    let re = Regex::new(r"\s+\[\[([^\]]+)\]\]\s*$").unwrap();
    let captures = re.captures(task_text)?;
    let full = captures.get(0)?;
    let source_id = captures.get(1)?.as_str().trim();
    let text = task_text[..full.start()].trim();
    if text.is_empty() || source_id.is_empty() {
        return None;
    }
    Some((text.to_string(), source_id.to_string()))
}

fn selection_key(source_id: &str, task_text: &str) -> String {
    let normalized = task_text.split_whitespace().collect::<Vec<_>>().join(" ");
    format!("{}\x1f{}", source_id.trim(), normalized)
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
    fn lists_active_task_index_todos() {
        let root = tempfile::tempdir().unwrap();
        let note = root.path().join("tasks.md");
        fs::write(
            note,
            r#"---
id: task-note
domains: [task-index]
task_active: true
task_area: Code
task_default_priority: p2
---

# Todo

## Next
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
domains: [task-index]
task_active: true
task_area: Action
---

# Productivity Beast

## Todo
### Next
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
        assert!(content.contains("## Reading\n- [ ] 35% Sacred Economics [[productivity-beast]]"));
        assert!(
            content.contains("## Code\n- [ ] Draft architecture review [[productivity-beast]]")
        );
        assert!(content.contains("## Action\n- [ ] Refill water bottle [[productivity-beast]]"));
    }

    #[test]
    fn sync_dashboard_applies_checkbox_writeback() {
        let root = tempfile::tempdir().unwrap();
        let note = root.path().join("hub.md");
        fs::write(
            &note,
            r#"---
id: productivity-beast
domains: [task-index]
task_active: true
task_area: Action
---

# Productivity Beast

## Todo
- [ ] Ship app fix area:Code
"#,
        )
        .unwrap();
        let dashboard = root.path().join("dashboard.md");
        sync_dashboard_from_task_index_notes(root.path(), &dashboard).unwrap();
        fs::write(
            &dashboard,
            "---\nid: \"dashboard\"\ntags: []\n---\n# Dashboard\n\n## Code\n- [x] Ship app fix [[productivity-beast]]\n",
        )
        .unwrap();

        let stats = sync_dashboard_from_task_index_notes(root.path(), &dashboard).unwrap();
        let source = fs::read_to_string(note).unwrap();
        let dashboard_content = fs::read_to_string(dashboard).unwrap();

        assert_eq!(stats.source_writebacks, 1);
        assert_eq!(stats.source_files_updated, 1);
        assert!(source.contains("- [x] Ship app fix area:Code"));
        assert!(dashboard_content.contains("## Code\n- [x] Ship app fix [[productivity-beast]]"));
    }
}
