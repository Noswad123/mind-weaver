use std::{collections::BTreeSet, fs, io, path::Path};

use crate::todos::is_managed_archive_document;
use crate::{
    ensure_meta_block_and_id, extract_metadata, has_meta_block, read_meta_id_from_content,
};

const HUB_FILE_NAME: &str = "hub.md";
const DEFAULT_HUB_CONTENT: &str = "---\ntags: []\n---\n\n## Topics\n## Research\n## Resources\n";
const REQUIRED_HEADERS: [&str; 3] = ["## Topics", "## Research", "## Resources"];
const TODO_HEADER: &str = "## Todo";
const TODO_CHILD_HEADERS: [&str; 3] = ["### Do Now", "### Inbox", "### Blocked"];
const CANONICAL_HUB_SECTIONS: [&str; 4] = ["Todo", "Topics", "Research", "Resources"];

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct FormatStats {
    pub hub_files_updated: usize,
    pub note_files_updated: usize,
    pub issues: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
struct DirSnapshot {
    dir_path: String,
    child_hub_ids: BTreeSet<String>,
    rel_note_ids: BTreeSet<String>,
}

pub fn format_notes(notes_root: impl AsRef<Path>, all_notes: bool) -> io::Result<FormatStats> {
    let root = notes_root.as_ref();
    if !root.is_dir() {
        return Err(io::Error::new(
            io::ErrorKind::NotFound,
            format!("notes directory missing: {}", root.display()),
        ));
    }

    let registry = crate::build_registry(root)?;
    if !registry.duplicates.is_empty() {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            "duplicate note IDs found; fix registry conflicts before formatting",
        ));
    }

    let mut stats = FormatStats::default();
    format_dirs(root, root, all_notes, &mut stats)?;
    Ok(stats)
}

fn format_dirs(
    dir: &Path,
    root: &Path,
    all_notes: bool,
    stats: &mut FormatStats,
) -> io::Result<()> {
    if is_hidden_dir(dir, root) {
        return Ok(());
    }

    if dir != root {
        let (changed, issues) = format_hub_note(dir)?;
        if changed {
            stats.hub_files_updated += 1;
        }
        stats.issues.extend(issues);
    }

    let mut child_dirs = Vec::new();
    let mut regular_notes = Vec::new();
    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        let path = entry.path();
        let file_type = entry.file_type()?;
        if file_type.is_dir() {
            if !is_hidden_name(&entry.file_name().to_string_lossy()) {
                child_dirs.push(path);
            }
            continue;
        }
        if all_notes && file_type.is_file() && is_markdown_file(&path) && !is_hub_note_path(&path) {
            regular_notes.push(path);
        }
    }

    child_dirs.sort();
    regular_notes.sort();
    for path in regular_notes {
        if format_regular_note(&path)? {
            stats.note_files_updated += 1;
        }
    }
    for child in child_dirs {
        format_dirs(&child, root, all_notes, stats)?;
    }
    Ok(())
}

fn format_hub_note(dir: &Path) -> io::Result<(bool, Vec<String>)> {
    let hub_path = dir.join(HUB_FILE_NAME);
    if !hub_path.exists() {
        fs::write(&hub_path, DEFAULT_HUB_CONTENT)?;
    }

    let original = fs::read_to_string(&hub_path)?;
    if is_managed_archive_document(&original) {
        return Ok((false, Vec::new()));
    }
    validate_task_note(&original)?;
    let (snapshot, mut issues) = build_snapshot(dir)?;
    let (updated, plan_issues) = plan_hub_update(&original, &snapshot);
    issues.extend(plan_issues);
    let changed = updated != trim_to_single_final_newline(&original);
    if changed {
        fs::write(&hub_path, updated)?;
    }
    Ok((changed, issues))
}

fn build_snapshot(dir: &Path) -> io::Result<(DirSnapshot, Vec<String>)> {
    let mut snapshot = DirSnapshot {
        dir_path: dir.to_string_lossy().to_string(),
        ..DirSnapshot::default()
    };
    let mut issues = Vec::new();

    let mut entries = Vec::new();
    for entry in fs::read_dir(dir)? {
        entries.push(entry?);
    }
    entries.sort_by_key(|entry| entry.file_name());

    for entry in entries {
        let name = entry.file_name().to_string_lossy().to_string();
        let path = entry.path();
        if entry.file_type()?.is_dir() {
            if is_hidden_name(&name) {
                continue;
            }
            let child_hub = path.join(HUB_FILE_NAME);
            if !child_hub.exists() {
                if let Err(err) = fs::write(&child_hub, DEFAULT_HUB_CONTENT) {
                    issues.push(format!(
                        "failed to ensure child hub {}: {err}",
                        child_hub.display()
                    ));
                }
            }
            if child_hub.exists() {
                let id = fs::read_to_string(&child_hub)
                    .ok()
                    .map(|content| read_meta_id_from_content(&content).0)
                    .filter(|id| !id.trim().is_empty())
                    .unwrap_or(name);
                snapshot.child_hub_ids.insert(id);
            }
            continue;
        }

        if is_markdown_file(&path) && !name.eq_ignore_ascii_case(HUB_FILE_NAME) {
            if let Some(stem) = path.file_stem().and_then(|stem| stem.to_str()) {
                snapshot.rel_note_ids.insert(stem.to_string());
            }
        }
    }

    Ok((snapshot, issues))
}

fn plan_hub_update(original: &str, snapshot: &DirSnapshot) -> (String, Vec<String>) {
    let mut issues = Vec::new();
    let mut content = original.to_string();
    if !has_meta_block(&content) {
        issues.push("missing frontmatter".to_string());
        content = format!(
            "---\nid: {:?}\ntags: []\n---\n\n{}",
            generate_hub_id(snapshot),
            content
        );
    } else if read_meta_id_from_content(&content).0.trim().is_empty() {
        issues.push("missing id in frontmatter".to_string());
        content = ensure_meta_block_and_id(&content, &generate_hub_id(snapshot)).0;
    }

    let is_task_index = has_task_index_domain(&content);
    let mut lines: Vec<String> = content.lines().map(ToString::to_string).collect();
    lines = normalize_hub_section_headings(&lines, is_task_index);
    lines = normalize_markdown_heading_lines(&lines, &hub_display_title(snapshot));
    let before_headers = lines.clone();
    if is_task_index {
        lines = ensure_task_index_todo_section(lines);
    }
    lines = ensure_headers(lines, &REQUIRED_HEADERS);
    if lines != before_headers {
        issues.push("missing required headers".to_string());
    }
    lines = reconcile_topics_section(lines, snapshot);
    (format!("{}\n", lines.join("\n").trim()), issues)
}

fn format_regular_note(path: &Path) -> io::Result<bool> {
    let original = fs::read_to_string(path)?;
    if is_managed_archive_document(&original) {
        return Ok(false);
    }
    validate_task_note(&original)?;
    let desired_id = path
        .file_stem()
        .and_then(|stem| stem.to_str())
        .unwrap_or("untitled");
    let with_id = ensure_meta_block_and_id(&original, desired_id).0;
    let title = path
        .file_stem()
        .and_then(|stem| stem.to_str())
        .unwrap_or("Untitled");
    let lines: Vec<String> = with_id.lines().map(ToString::to_string).collect();
    let normalized = normalize_markdown_heading_lines(&lines, title);
    let updated = format!("{}\n", normalized.join("\n").trim_end());
    let changed = updated != original;
    if changed {
        fs::write(path, updated)?;
    }
    Ok(changed)
}

fn normalize_hub_section_headings(lines: &[String], include_todo: bool) -> Vec<String> {
    let mut out = lines.to_vec();
    let mut in_fence = false;
    for line in &mut out {
        let trimmed = line.trim();
        if is_fence_delimiter(trimmed) {
            in_fence = !in_fence;
            continue;
        }
        if in_fence {
            continue;
        }
        let Some((_level, heading)) = parse_hash_heading(trimmed) else {
            continue;
        };
        let Some(canonical) = canonical_hub_section_name(&heading, include_todo) else {
            continue;
        };
        *line = format!("## {canonical}");
    }
    out
}

fn ensure_task_index_todo_section(lines: Vec<String>) -> Vec<String> {
    if section_bounds(&lines, TODO_HEADER).is_some() {
        return ensure_todo_child_headers(lines);
    }

    let insert_at = lines
        .iter()
        .position(|line| REQUIRED_HEADERS.iter().any(|header| line.trim() == *header))
        .unwrap_or(lines.len());

    let mut out = Vec::with_capacity(lines.len() + 8);
    out.extend_from_slice(&lines[..insert_at]);
    if out.last().is_some_and(|line| !line.trim().is_empty()) {
        out.push(String::new());
    }
    out.push(TODO_HEADER.to_string());
    out.push(TODO_CHILD_HEADERS[0].to_string());
    out.push(String::new());
    out.push(TODO_CHILD_HEADERS[1].to_string());
    out.push(String::new());
    out.push(TODO_CHILD_HEADERS[2].to_string());
    if insert_at < lines.len() {
        out.push(String::new());
        out.extend_from_slice(&lines[insert_at..]);
    }
    out
}

fn ensure_todo_child_headers(lines: Vec<String>) -> Vec<String> {
    let Some((start, end)) = todo_section_bounds(&lines) else {
        return lines;
    };
    let mut body = lines[start + 1..end].to_vec();
    let seen: BTreeSet<String> = body.iter().map(|line| line.trim().to_string()).collect();
    for header in TODO_CHILD_HEADERS {
        if seen.contains(header) {
            continue;
        }
        if body.last().is_some_and(|line| !line.trim().is_empty()) {
            body.push(String::new());
        }
        body.push(header.to_string());
    }

    let mut out = lines[..=start].to_vec();
    out.extend(body);
    if end < lines.len() {
        if out.last().is_some_and(|line| !line.trim().is_empty()) {
            out.push(String::new());
        }
        out.extend_from_slice(&lines[end..]);
    }
    out
}

pub fn normalize_markdown_headings(content: &str, title: &str) -> String {
    if is_managed_archive_document(content) {
        return content.to_string();
    }
    let lines: Vec<String> = content.lines().map(ToString::to_string).collect();
    format!(
        "{}\n",
        normalize_markdown_heading_lines(&lines, title).join("\n")
    )
}

fn normalize_markdown_heading_lines(lines: &[String], title: &str) -> Vec<String> {
    let mut out = lines.to_vec();
    let mut in_fence = false;
    let mut h1_seen = false;
    for line in &mut out {
        let trimmed = line.trim();
        if is_fence_delimiter(trimmed) {
            in_fence = !in_fence;
            continue;
        }
        if in_fence {
            continue;
        }
        let Some((level, heading)) = parse_hash_heading(trimmed) else {
            continue;
        };
        if level != 1 {
            continue;
        }
        if !h1_seen {
            h1_seen = true;
            continue;
        }
        *line = rewrite_heading_line(line, 2, &heading);
    }

    if h1_seen {
        return out;
    }
    let insert_at = index_after_frontmatter(&out);
    let title = if title.trim().is_empty() {
        "Untitled"
    } else {
        title.trim()
    };
    let mut rebuilt = Vec::with_capacity(out.len() + 2);
    rebuilt.extend_from_slice(&out[..insert_at]);
    if !rebuilt.is_empty() && rebuilt.last().is_some_and(|line| !line.trim().is_empty()) {
        rebuilt.push(String::new());
    }
    rebuilt.push(format!("# {title}"));
    if insert_at < out.len() && !out[insert_at].trim().is_empty() {
        rebuilt.push(String::new());
    }
    rebuilt.extend_from_slice(&out[insert_at..]);
    rebuilt
}

fn ensure_headers(mut lines: Vec<String>, headers: &[&str]) -> Vec<String> {
    let seen: BTreeSet<String> = lines.iter().map(|line| line.trim().to_string()).collect();
    for header in headers {
        if seen.contains(*header) {
            continue;
        }
        if !lines.is_empty() && lines.last().is_some_and(|line| !line.trim().is_empty()) {
            lines.push(String::new());
        }
        lines.push((*header).to_string());
        lines.push(String::new());
    }
    lines
}

fn reconcile_topics_section(lines: Vec<String>, snapshot: &DirSnapshot) -> Vec<String> {
    let Some((start, end)) = section_bounds(&lines, "## Topics") else {
        return lines;
    };
    let allowed: BTreeSet<String> = snapshot
        .child_hub_ids
        .iter()
        .chain(snapshot.rel_note_ids.iter())
        .cloned()
        .collect();
    let mut seen = BTreeSet::new();
    let mut cleaned = Vec::new();
    for line in &lines[start + 1..end] {
        let Some(id) = topic_link_id(line) else {
            continue;
        };
        if allowed.contains(&id) && seen.insert(id) {
            cleaned.push(line.clone());
        }
    }
    for id in allowed {
        if seen.insert(id.clone()) {
            cleaned.push(format!("- [[{id}]]"));
        }
    }
    replace_section_body(lines, start, end, cleaned)
}

fn topic_link_id(line: &str) -> Option<String> {
    let trimmed = line.trim();
    for prefix in ["- [[", "* [["] {
        if let Some(rest) = trimmed.strip_prefix(prefix) {
            let id = rest.strip_suffix("]]")?.trim();
            if !id.is_empty() {
                return Some(id.to_string());
            }
        }
    }
    for prefix in ["- {:", "* {:"] {
        if let Some(rest) = trimmed.strip_prefix(prefix) {
            let id = rest.strip_suffix(":}")?.trim();
            if !id.is_empty() {
                return Some(id.to_string());
            }
        }
    }
    None
}

fn section_bounds(lines: &[String], section: &str) -> Option<(usize, usize)> {
    let start = lines.iter().position(|line| line.trim() == section)?;
    let end = lines[start + 1..]
        .iter()
        .position(|line| parse_hash_heading(line.trim()).is_some())
        .map(|offset| start + 1 + offset)
        .unwrap_or(lines.len());
    Some((start, end))
}

// Todo scaffolding owns its child headings. Other reconcilers deliberately stop
// at the first heading so they cannot replace a user's nested notes/prose.
fn todo_section_bounds(lines: &[String]) -> Option<(usize, usize)> {
    let start = lines.iter().position(|line| line.trim() == TODO_HEADER)?;
    let end = lines[start + 1..]
        .iter()
        .position(|line| parse_hash_heading(line.trim()).is_some_and(|(level, _)| level <= 2))
        .map(|offset| start + 1 + offset)
        .unwrap_or(lines.len());
    Some((start, end))
}

fn replace_section_body(
    lines: Vec<String>,
    start: usize,
    end: usize,
    entries: Vec<String>,
) -> Vec<String> {
    let mut suffix = lines[end..].to_vec();
    while suffix.first().is_some_and(|line| line.trim().is_empty()) {
        suffix.remove(0);
    }
    let mut out = lines[..=start].to_vec();
    out.extend(entries);
    if !suffix.is_empty() {
        out.push(String::new());
    }
    out.extend(suffix);
    out
}

fn parse_hash_heading(line: &str) -> Option<(usize, String)> {
    if !line.starts_with('#') {
        return None;
    }
    let level = line.chars().take_while(|ch| *ch == '#').count();
    let rest = line.get(level..)?;
    if !rest.starts_with(' ') {
        return None;
    }
    let heading = rest.trim().to_string();
    (!heading.is_empty()).then_some((level, heading))
}

fn canonical_hub_section_name(name: &str, include_todo: bool) -> Option<&'static str> {
    CANONICAL_HUB_SECTIONS
        .iter()
        .copied()
        .filter(|section| include_todo || !section.eq_ignore_ascii_case("Todo"))
        .find(|section| section.eq_ignore_ascii_case(name.trim()))
}

fn has_task_index_domain(content: &str) -> bool {
    extract_metadata(content)
        .domains
        .iter()
        .any(|domain| domain.eq_ignore_ascii_case("task"))
}

fn validate_task_note(content: &str) -> io::Result<()> {
    let domains = extract_metadata(content).domains;
    if domains.iter().any(|domain| {
        domain.eq_ignore_ascii_case("task-index") || domain.eq_ignore_ascii_case("taskIndex")
    }) {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            "legacy task domain is not supported; migrate task-index/taskIndex to task and Todo headings to Do Now, Inbox, Blocked before formatting",
        ));
    }
    if has_task_index_domain(content) {
        crate::todos::validate_task_vocabulary(content)?;
    }
    Ok(())
}

fn rewrite_heading_line(line: &str, level: usize, heading: &str) -> String {
    let leading_len = line.len() - line.trim_start_matches([' ', '\t']).len();
    let prefix = &line[..leading_len];
    let heading = if heading.trim().is_empty() {
        "Untitled"
    } else {
        heading.trim()
    };
    format!("{}{} {heading}", prefix, "#".repeat(level))
}

fn index_after_frontmatter(lines: &[String]) -> usize {
    let mut idx = 0;
    while idx < lines.len() && lines[idx].trim().is_empty() {
        idx += 1;
    }
    if idx >= lines.len() || lines[idx].trim() != "---" {
        return idx;
    }
    for i in idx + 1..lines.len() {
        if lines[i].trim() == "---" {
            idx = i + 1;
            while idx < lines.len() && lines[idx].trim().is_empty() {
                idx += 1;
            }
            return idx;
        }
    }
    idx
}

fn hub_display_title(snapshot: &DirSnapshot) -> String {
    let title = Path::new(&snapshot.dir_path)
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("Hub")
        .replace(['-', '_'], " ");
    let title = title.split_whitespace().collect::<Vec<_>>().join(" ");
    if title.is_empty() {
        "Hub".to_string()
    } else {
        title
    }
}

fn generate_hub_id(snapshot: &DirSnapshot) -> String {
    Path::new(&snapshot.dir_path)
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("hub")
        .to_string()
}

fn is_hidden_dir(dir: &Path, root: &Path) -> bool {
    dir != root
        && dir
            .file_name()
            .and_then(|name| name.to_str())
            .is_some_and(is_hidden_name)
}

fn is_hidden_name(name: &str) -> bool {
    name == ".git" || name.starts_with('.')
}

fn is_markdown_file(path: &Path) -> bool {
    path.extension()
        .and_then(|ext| ext.to_str())
        .is_some_and(|ext| ext.eq_ignore_ascii_case("md"))
}

fn is_hub_note_path(path: &Path) -> bool {
    path.file_name()
        .and_then(|name| name.to_str())
        .is_some_and(|name| name.eq_ignore_ascii_case(HUB_FILE_NAME))
}

fn is_fence_delimiter(trimmed: &str) -> bool {
    trimmed.starts_with("```") || trimmed.starts_with("~~~")
}

fn trim_to_single_final_newline(content: &str) -> String {
    format!("{}\n", content.trim())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn todo_scaffolding_does_not_expand_topics_reconciliation_ownership() {
        let root = tempfile::tempdir().unwrap();
        let hub = root.path().join("hub.md");
        let nested =
            "### Working notes\nImportant prose that must survive.\n- [ ] An unrelated checklist\n";
        let content = format!(
            "---\nid: hub\ndomains: [task]\n---\n# Hub\n## Todo\n### Do Now\n### Inbox\n### Blocked\n## Topics\n{nested}## Research\n## Resources\n"
        );
        fs::write(&hub, content).unwrap();
        format_hub_note(root.path()).unwrap();
        let once = fs::read_to_string(&hub).unwrap();
        assert!(once.contains(nested));
        format_hub_note(root.path()).unwrap();
        assert_eq!(fs::read_to_string(hub).unwrap(), once);
    }

    #[test]
    fn formatting_rejects_legacy_task_vocabulary_without_rewriting_notes() {
        let root = tempfile::tempdir().unwrap();
        let path = root.path().join("note.md");
        for (domain, heading) in [
            ("task-index", "Next"),
            ("taskIndex", "Waiting"),
            ("task", "Done"),
        ] {
            let content = format!(
                "---\nid: note\ndomains: [{domain}]\n---\n# Note\n## Todo\n### {heading}\n- [ ] Work\n"
            );
            fs::write(&path, &content).unwrap();
            assert!(
                format_regular_note(&path)
                    .unwrap_err()
                    .to_string()
                    .contains("migrate")
            );
            assert_eq!(fs::read_to_string(&path).unwrap(), content);
            let hub = root.path().join("hub.md");
            fs::write(&hub, &content).unwrap();
            assert!(
                format_hub_note(root.path())
                    .unwrap_err()
                    .to_string()
                    .contains("migrate")
            );
            assert_eq!(fs::read_to_string(hub).unwrap(), content);
        }
    }

    #[test]
    fn canonical_todo_children_are_not_duplicated_when_formatting_again() {
        let content = "---\nid: tasks\ndomains: [task]\n---\n# Tasks\n## Todo\n### Do Now\n- [ ] Work\n### Inbox\n### Blocked\n## Topics\n## Research\n## Resources\n";
        let snapshot = DirSnapshot::default();
        let (once, _) = plan_hub_update(content, &snapshot);
        let (twice, _) = plan_hub_update(&once, &snapshot);
        assert_eq!(once, twice);
        for heading in TODO_CHILD_HEADERS {
            assert_eq!(once.matches(heading).count(), 1);
        }
    }

    #[test]
    fn formats_regular_note_with_id_and_single_h1() {
        let temp = tempfile::tempdir().unwrap();
        let path = temp.path().join("idea.md");
        fs::write(&path, "Body\n# First\n# Second\n").unwrap();

        assert!(format_regular_note(&path).unwrap());
        let updated = fs::read_to_string(&path).unwrap();
        assert!(updated.starts_with("---\nid: \"idea\"\n---"));
        assert!(updated.contains("# First"));
        assert!(updated.contains("## Second"));
    }

    #[test]
    fn formats_hub_topics_from_child_hubs_and_notes() {
        let temp = tempfile::tempdir().unwrap();
        let root = temp.path();
        let area = root.join("projects");
        let child = area.join("alpha");
        fs::create_dir_all(&child).unwrap();
        fs::write(area.join("note.md"), "# Note\n").unwrap();

        let stats = format_notes(root, false).unwrap();
        assert!(stats.hub_files_updated >= 1);
        let hub = fs::read_to_string(area.join("hub.md")).unwrap();
        assert!(hub.contains("id: \"projects\""));
        assert!(hub.contains("# projects"));
        assert!(hub.contains("## Topics"));
        assert!(hub.contains("- [[alpha]]"));
        assert!(hub.contains("- [[note]]"));
        assert!(child.join("hub.md").exists());
    }

    #[test]
    fn non_task_index_hub_does_not_get_todo_section() {
        let temp = tempfile::tempdir().unwrap();
        let root = temp.path();
        let area = root.join("projects");
        fs::create_dir_all(&area).unwrap();
        fs::write(
            area.join("hub.md"),
            r#"---
id: "projects"
tags: []
---

# Projects

## Topics
## Research
## Resources
"#,
        )
        .unwrap();

        format_notes(root, false).unwrap();
        let hub = fs::read_to_string(area.join("hub.md")).unwrap();

        assert!(!hub.contains("## Todo"));
    }

    #[test]
    fn task_index_hub_gets_todo_workflow_section() {
        let temp = tempfile::tempdir().unwrap();
        let root = temp.path();
        let area = root.join("projects");
        fs::create_dir_all(&area).unwrap();
        fs::write(
            area.join("hub.md"),
            r#"---
id: "projects"
tags: []
domains: [task]
task_active: true
---

# Projects

## Topics
## Research
## Resources
"#,
        )
        .unwrap();

        format_notes(root, false).unwrap();
        let hub = fs::read_to_string(area.join("hub.md")).unwrap();

        assert!(hub.contains("## Todo\n### Do Now\n\n### Inbox\n\n### Blocked"));
        assert!(hub.find("## Todo") < hub.find("## Topics"));
    }

    #[test]
    fn reserved_archive_documents_are_opaque_even_with_invalid_or_future_bodies() {
        let temp = tempfile::tempdir().unwrap();
        let regular = temp.path().join("archive.md");
        let area = temp.path().join("area");
        fs::create_dir(&area).unwrap();
        for body in [
            "<!-- mw-archive-ledger:v99 -->\nunsupported future protocol\r\n# Raw\n# Other",
            "<!-- mw-archive-ledger:v1 -->\nCORRUPT receipt\n  ```\n# Raw\r\n- [ ] payload\n",
        ] {
            let content = format!(
                "---\nid: mw-archive-0123456789abcdef0123456789abcdef\ntask_active: false\n---\n{body}"
            );
            let hub_content = content.replace(
                "0123456789abcdef0123456789abcdef",
                "0123456789abcdef0123456789abcdee",
            );
            fs::write(&regular, &content).unwrap();
            fs::write(area.join("hub.md"), &hub_content).unwrap();
            assert!(!format_regular_note(&regular).unwrap());
            assert!(!format_hub_note(&area).unwrap().0);
            assert_eq!(fs::read(&regular).unwrap(), content.as_bytes());
            assert_eq!(
                fs::read(area.join("hub.md")).unwrap(),
                hub_content.as_bytes()
            );
            assert_eq!(
                normalize_markdown_headings(&content, "must not insert title"),
                content
            );
            format_notes(temp.path(), true).unwrap();
            assert_eq!(fs::read(&regular).unwrap(), content.as_bytes());
            assert_eq!(
                fs::read(area.join("hub.md")).unwrap(),
                hub_content.as_bytes()
            );
        }
    }

    #[test]
    fn archive_examples_and_nonreserved_note_ids_still_format_normally() {
        let temp = tempfile::tempdir().unwrap();
        for (index, content) in [
            "---\nid: regular\n---\n```markdown\n---\nid: mw-archive-0123456789abcdef0123456789abcdef\n---\n<!-- mw-archive-ledger:v1 -->\n```\n",
            "---\nid: regular\ndescription: |\n  id: mw-archive-0123456789abcdef0123456789abcdef\n---\n<!-- mw-archive-ledger:v1 -->\nBody\n",
            "---\nid: mw-archive-not-a-reserved-id\n---\nBody\n",
            "```markdown\n---\nid: mw-archive-0123456789abcdef0123456789abcdef\n---\n<!-- mw-archive-ledger:v1 -->\n```\n",
        ].iter().enumerate() {
            let path = temp.path().join(format!("note-{index}.md"));
            fs::write(&path, content).unwrap();
            assert!(!is_managed_archive_document(content));
            assert!(format_regular_note(&path).unwrap());
            let updated = fs::read_to_string(path).unwrap();
            assert!(updated.contains(&format!("# note-{index}")));
            assert_ne!(updated, *content);
        }
    }

    #[test]
    fn archive_namespace_recognition_is_frontmatter_scoped_not_ledger_validation() {
        for id in [
            "mw-archive-0123456789abcdef0123456789abcdef",
            "\"mw-archive-0123456789abcdef0123456789abcdef\"",
            "'mw-archive-0123456789abcdef0123456789abcdef' # reserved",
        ] {
            for newline in ["\n", "\r\n"] {
                let content =
                    format!("---\nid: {id}\ntask_active: false\n---\nnot even a ledger body")
                        .replace('\n', newline);
                assert!(is_managed_archive_document(&content));
                assert_eq!(normalize_markdown_headings(&content, "unused"), content);
            }
        }
    }
}
