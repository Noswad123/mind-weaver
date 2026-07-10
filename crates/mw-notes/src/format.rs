use std::{collections::BTreeSet, fs, io, path::Path};

use crate::{ensure_meta_block_and_id, has_meta_block, read_meta_id_from_content};

const HUB_FILE_NAME: &str = "hub.md";
const DEFAULT_HUB_CONTENT: &str = "---\ntags: []\n---\n\n## Topics\n## Research\n## Resources\n";
const REQUIRED_HEADERS: [&str; 3] = ["## Topics", "## Research", "## Resources"];
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

    let (snapshot, mut issues) = build_snapshot(dir)?;
    let original = fs::read_to_string(&hub_path)?;
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

    let mut lines: Vec<String> = content.lines().map(ToString::to_string).collect();
    lines = normalize_hub_section_headings(&lines);
    lines = normalize_markdown_heading_lines(&lines, &hub_display_title(snapshot));
    let before_headers = lines.clone();
    lines = ensure_headers(lines, &REQUIRED_HEADERS);
    if lines != before_headers {
        issues.push("missing required headers".to_string());
    }
    lines = reconcile_topics_section(lines, snapshot);
    (format!("{}\n", lines.join("\n").trim()), issues)
}

fn format_regular_note(path: &Path) -> io::Result<bool> {
    let original = fs::read_to_string(path)?;
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

fn normalize_hub_section_headings(lines: &[String]) -> Vec<String> {
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
        let Some(canonical) = canonical_hub_section_name(&heading) else {
            continue;
        };
        *line = format!("## {canonical}");
    }
    out
}

pub fn normalize_markdown_headings(content: &str, title: &str) -> String {
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
        .position(|line| line.trim().starts_with('#'))
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

fn canonical_hub_section_name(name: &str) -> Option<&'static str> {
    CANONICAL_HUB_SECTIONS
        .iter()
        .copied()
        .find(|section| section.eq_ignore_ascii_case(name.trim()))
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
}
