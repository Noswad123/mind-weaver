use std::{collections::BTreeMap, path::Path};

use regex::Regex;

use crate::{Link, ParseContext, extract_metadata, parse_links_with_context};

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct ParsedNote {
    pub title: String,
    pub domains: Vec<String>,
    pub tags: Vec<String>,
    pub meta: BTreeMap<String, String>,
    pub links: Vec<Link>,
    pub content: String,
}

pub fn parse_note(content: &str, file_path: &str) -> ParsedNote {
    parse_note_with_context(
        content,
        &ParseContext {
            source_rel_path: file_path.to_string(),
            notes_root_abs: String::new(),
        },
    )
}

pub fn parse_note_with_context(content: &str, ctx: &ParseContext) -> ParsedNote {
    let title = title_from_path(&ctx.source_rel_path);
    let metadata = extract_metadata(content);

    let mut tags = metadata.tags.clone();
    let tag_pattern = Regex::new(r":([a-zA-Z0-9_-]+):").expect("valid tag regex");
    for captures in tag_pattern.captures_iter(content) {
        if let Some(tag) = captures.get(1) {
            tags.push(tag.as_str().to_string());
        }
    }

    ParsedNote {
        title,
        domains: metadata.domains,
        tags,
        meta: metadata.raw,
        links: parse_links_with_context(content, ctx),
        content: content.to_string(),
    }
}

fn title_from_path(path: &str) -> String {
    let normalized = path.replace('\\', "/");
    let file_name = normalized.rsplit('/').next().unwrap_or(&normalized);
    Path::new(file_name)
        .file_stem()
        .and_then(|stem| stem.to_str())
        .unwrap_or(file_name)
        .to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_note_metadata_tags_and_links() {
        let parsed = parse_note(
            r#"---
domains: [glossary, biology]
tags: [term, biology]
id: "Immunity"
---

# Immunity :science:

See [[cells|Cells]] and [Local](child.md).
"#,
            "areas/biology/immunity.md",
        );

        assert_eq!(parsed.title, "immunity");
        assert_eq!(parsed.domains, vec!["glossary", "biology"]);
        assert_eq!(parsed.tags, vec!["term", "biology", "science"]);
        assert_eq!(parsed.meta.get("id"), Some(&"Immunity".to_string()));
        assert_eq!(parsed.links.len(), 2);
        assert_eq!(parsed.links[0].target, "cells");
        assert_eq!(parsed.links[1].resolved_path, "areas/biology/child.md");
    }
}
