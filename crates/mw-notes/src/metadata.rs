use std::collections::BTreeMap;

use regex::Regex;

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct Metadata {
    pub tags: Vec<String>,
    pub domains: Vec<String>,
    pub raw: BTreeMap<String, String>,
}

pub fn extract_metadata(content: &str) -> Metadata {
    let mut metadata = Metadata::default();
    let Some(block) = frontmatter_block_strict(content) else {
        return metadata;
    };

    for line in block.lines() {
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }

        let Some((key, value)) = parse_yaml_key_value(line) else {
            continue;
        };

        match key.as_str() {
            "tags" => metadata.tags = parse_list_like_value(&value),
            "domains" => metadata.domains = parse_list_like_value(&value),
            _ => {
                metadata.raw.insert(key, value);
            }
        }
    }

    metadata
}

pub fn parse_list_like_value(value: &str) -> Vec<String> {
    let value = value.trim();
    if value.is_empty() {
        return vec![];
    }

    if value.starts_with('[') && value.ends_with(']') {
        let inner = value[1..value.len() - 1].trim();
        if inner.is_empty() {
            return vec![];
        }
        return split_csv_loose(inner)
            .into_iter()
            .filter_map(|part| clean_list_item(&part))
            .collect();
    }

    value.split(',').filter_map(clean_list_item).collect()
}

pub fn has_meta_block(content: &str) -> bool {
    frontmatter_block_lenient(content).is_some()
}

pub fn ensure_meta_block(content: &str) -> String {
    if has_meta_block(content) {
        content.to_string()
    } else {
        format!("---\n---\n\n{content}")
    }
}

pub fn ensure_meta_id(content: &str, id: &str) -> String {
    let Some((start, end, block)) = frontmatter_block_lenient_with_span(content) else {
        return content.to_string();
    };

    let updated_block = if meta_id_line(&block).is_some() {
        if read_meta_id_from_block(&block).is_some_and(|value| !value.trim().is_empty()) {
            block
        } else {
            replace_empty_meta_id_line(&block, id)
        }
    } else {
        insert_meta_id_line(&block, id)
    };

    let mut out = String::with_capacity(content.len() + id.len() + 8);
    out.push_str(&content[..start]);
    out.push_str(&updated_block);
    out.push_str(&content[end..]);
    out
}

pub fn ensure_meta_block_and_id(content: &str, desired_id: &str) -> (String, bool) {
    let original = content.to_string();
    let with_block = ensure_meta_block(content);
    let updated = ensure_meta_id(&with_block, desired_id);
    let changed = updated != original;
    (updated, changed)
}

pub fn read_meta_id_from_content(content: &str) -> (String, bool) {
    let Some(block) = frontmatter_block_lenient(content) else {
        return (String::new(), false);
    };
    match read_meta_id_from_block(&block) {
        Some(value) => (value.trim().to_string(), true),
        None => (String::new(), true),
    }
}

pub fn validate_meta_id(content: &str, desired_id: &str) -> (bool, String) {
    let original = format!("{}\n", content.trim());
    let (updated, _) = ensure_meta_block_and_id(content, desired_id);
    let updated = format!("{}\n", updated.trim());

    if updated == original {
        (false, String::new())
    } else {
        (
            true,
            "note is missing frontmatter id (run `mw notes format --all` to fix)".to_string(),
        )
    }
}

fn frontmatter_block_strict(content: &str) -> Option<String> {
    let re = Regex::new(r"(?s)^---\s*\n(.*?)\n---").expect("valid frontmatter regex");
    re.captures(content)
        .and_then(|captures| captures.get(1).map(|m| m.as_str().to_string()))
}

fn frontmatter_block_lenient(content: &str) -> Option<String> {
    frontmatter_block_lenient_with_span(content).map(|(_, _, block)| block)
}

fn frontmatter_block_lenient_with_span(content: &str) -> Option<(usize, usize, String)> {
    let re = Regex::new(r"(?s)^---\s*\n(.*?)\n?---\s*(?:\n|$)").expect("valid meta block regex");
    let captures = re.captures(content)?;
    let full = captures.get(0)?;
    Some((full.start(), full.end(), full.as_str().to_string()))
}

fn parse_yaml_key_value(line: &str) -> Option<(String, String)> {
    let (key, raw_value) = line.split_once(':')?;
    let key = key.trim();
    if key.is_empty()
        || !key
            .chars()
            .all(|ch| ch.is_ascii_alphanumeric() || ch == '_' || ch == '-')
    {
        return None;
    }

    let raw_value = raw_value.trim_start();
    let value = if let Some(rest) = raw_value.strip_prefix('"') {
        rest.split_once('"').map(|(v, _)| v.to_string())?
    } else if let Some(rest) = raw_value.strip_prefix('\'') {
        rest.split_once('\'').map(|(v, _)| v.to_string())?
    } else {
        raw_value
            .split_once('#')
            .map(|(v, _)| v)
            .unwrap_or(raw_value)
            .trim()
            .to_string()
    };

    Some((key.to_string(), value.trim().to_string()))
}

fn split_csv_loose(value: &str) -> Vec<String> {
    let mut parts = Vec::new();
    let mut buf = String::new();
    let mut in_single = false;
    let mut in_double = false;

    let flush = |parts: &mut Vec<String>, buf: &mut String| {
        let piece = buf.trim().to_string();
        buf.clear();
        if !piece.is_empty() {
            parts.push(piece);
        }
    };

    for ch in value.chars() {
        match ch {
            '\'' => {
                if !in_double {
                    in_single = !in_single;
                }
                buf.push(ch);
            }
            '"' => {
                if !in_single {
                    in_double = !in_double;
                }
                buf.push(ch);
            }
            ',' if !in_single && !in_double => flush(&mut parts, &mut buf),
            _ => buf.push(ch),
        }
    }
    flush(&mut parts, &mut buf);

    parts
}

fn clean_list_item(item: impl AsRef<str>) -> Option<String> {
    let item = item.as_ref().trim().trim_matches(['"', '\'']).trim();
    if item.is_empty() {
        None
    } else {
        Some(item.to_string())
    }
}

fn meta_id_line(block: &str) -> Option<&str> {
    block
        .lines()
        .find(|line| parse_meta_id_line(line).is_some())
}

fn read_meta_id_from_block(block: &str) -> Option<String> {
    block.lines().find_map(parse_meta_id_line)
}

fn parse_meta_id_line(line: &str) -> Option<String> {
    let trimmed = line.trim_start();
    if !trimmed.starts_with("id") {
        return None;
    }
    let after_id = &trimmed[2..];
    let after_id = after_id.trim_start();
    let after_sep = after_id
        .strip_prefix(':')
        .or_else(|| after_id.strip_prefix('='))?;
    let value = after_sep.trim_start();

    if let Some(rest) = value.strip_prefix('"') {
        return rest
            .split_once('"')
            .map(|(v, _)| v.to_string())
            .or(Some(String::new()));
    }
    if let Some(rest) = value.strip_prefix('\'') {
        return rest
            .split_once('\'')
            .map(|(v, _)| v.to_string())
            .or(Some(String::new()));
    }

    let value = value
        .split_once('#')
        .map(|(v, _)| v)
        .unwrap_or(value)
        .trim();
    Some(
        value
            .split_whitespace()
            .next()
            .unwrap_or_default()
            .to_string(),
    )
}

fn replace_empty_meta_id_line(block: &str, id: &str) -> String {
    block
        .lines()
        .map(|line| {
            if parse_meta_id_line(line).is_some() && read_id_has_no_value(line) {
                format!("id: {:?}", id)
            } else {
                line.to_string()
            }
        })
        .collect::<Vec<_>>()
        .join("\n")
}

fn read_id_has_no_value(line: &str) -> bool {
    parse_meta_id_line(line).is_some_and(|value| value.trim().is_empty())
}

fn insert_meta_id_line(block: &str, id: &str) -> String {
    let mut lines = Vec::new();
    let mut inserted = false;
    for (idx, line) in block.lines().enumerate() {
        lines.push(line.to_string());
        if !inserted && idx == 0 && line.trim() == "---" {
            lines.push(format!("id: {:?}", id));
            inserted = true;
        }
    }
    lines.join("\n")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn extract_metadata_parses_domains_and_tags() {
        let content = r#"---
domains: [glossary, biology]
tags: "['term:immunity', 'topic:biology']"
id: "Immunity"
---

# Immunity
"#;

        let metadata = extract_metadata(content);

        assert_eq!(metadata.domains, vec!["glossary", "biology"]);
        assert_eq!(metadata.tags, vec!["term:immunity", "topic:biology"]);
        assert_eq!(metadata.raw.get("id"), Some(&"Immunity".to_string()));
    }

    #[test]
    fn parse_list_like_value_supports_bracket_and_csv() {
        assert_eq!(
            parse_list_like_value("[glossary, biology]"),
            vec!["glossary", "biology"]
        );
        assert_eq!(
            parse_list_like_value("['tool:git', 'topic:vcs']"),
            vec!["tool:git", "topic:vcs"]
        );
        assert_eq!(
            parse_list_like_value("one, two, three"),
            vec!["one", "two", "three"]
        );
    }

    #[test]
    fn ensure_meta_id_replaces_empty_id_line() {
        let updated = ensure_meta_id("---\nid:\n---\n\n# Note\n", "note-id");
        let (id, ok) = read_meta_id_from_content(&updated);
        assert!(ok);
        assert_eq!(id, "note-id");
        assert_eq!(updated.matches("id:").count(), 1);
    }

    #[test]
    fn ensure_meta_id_inserts_id_into_empty_frontmatter_block() {
        let updated = ensure_meta_id("---\n---\n\n# Note\n", "note-id");
        let (id, ok) = read_meta_id_from_content(&updated);
        assert!(ok);
        assert_eq!(id, "note-id");
    }

    #[test]
    fn ensure_meta_block_and_id_reports_changes() {
        let (updated, changed) = ensure_meta_block_and_id("# Note\n", "note-id");
        assert!(changed);
        assert!(updated.starts_with("---\nid: \"note-id\"\n---"));
    }

    #[test]
    fn read_meta_id_reports_missing_id_when_block_exists() {
        let (id, ok) = read_meta_id_from_content("---\ntags: []\n---\n");
        assert!(ok);
        assert_eq!(id, "");
    }
}
