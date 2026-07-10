use std::{env, path::Path};

use regex::Regex;

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ParseContext {
    pub source_rel_path: String,
    pub notes_root_abs: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum LinkType {
    Internal,
    External,
}

impl Default for LinkType {
    fn default() -> Self {
        Self::Internal
    }
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct Link {
    #[serde(rename = "type")]
    pub link_type: LinkType,
    pub target: String,
    pub label: String,
    pub resolved_path: String,
}

pub fn parse_links(content: &str, source_path: &str) -> Vec<Link> {
    parse_links_with_context(
        content,
        &ParseContext {
            source_rel_path: source_path.to_string(),
            notes_root_abs: String::new(),
        },
    )
}

pub fn parse_links_with_context(content: &str, ctx: &ParseContext) -> Vec<Link> {
    let mut links = Vec::new();

    // Pattern A: {:label:}[link], Pattern B: [link]{:label:}
    let internal = Regex::new(r#"\{:([^}]+):}\[([^\]]+)]|\[([^\]]+)]\{:([^}]+):}"#)
        .expect("valid internal link regex");
    let external = Regex::new(r#"\{(https?://[^}]+)}\[([^\]]+)]|\[([^\]]+)]\{(https?://[^}]+)}"#)
        .expect("valid external link regex");
    let wiki_link = Regex::new(r#"\[\[([^\]|#]+(?:#[^\]|]+)?)(?:\|([^\]]+))?\]\]"#)
        .expect("valid wiki link regex");
    let markdown_link =
        Regex::new(r#"(!?)\[([^\]]+)]\(([^)]+)\)"#).expect("valid markdown link regex");

    for captures in internal.captures_iter(content) {
        let raw_path = capture_first(&captures, &[1, 4]);
        let label = capture_first(&captures, &[2, 3]);
        links.push(Link {
            link_type: LinkType::Internal,
            target: raw_path.clone(),
            label,
            resolved_path: resolve_internal_link(&raw_path),
        });
    }

    for captures in external.captures_iter(content) {
        let url = capture_first(&captures, &[1, 4]);
        let label = capture_first(&captures, &[2, 3]);
        links.push(Link {
            link_type: LinkType::External,
            target: url,
            label,
            resolved_path: String::new(),
        });
    }

    for captures in wiki_link.captures_iter(content) {
        let raw_target =
            normalize_wiki_link_target(captures.get(1).map(|m| m.as_str()).unwrap_or_default());
        if raw_target.is_empty() {
            continue;
        }
        let label = captures
            .get(2)
            .map(|m| m.as_str().trim().to_string())
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| raw_target.clone());
        links.push(Link {
            link_type: LinkType::Internal,
            target: raw_target.clone(),
            label,
            resolved_path: resolve_markdown_local_link_with_context(&raw_target, ctx),
        });
    }

    for captures in markdown_link.captures_iter(content) {
        if captures.get(1).map(|m| m.as_str()).unwrap_or_default() == "!" {
            continue;
        }
        let label = captures
            .get(2)
            .map(|m| m.as_str().trim().to_string())
            .unwrap_or_default();
        let target = captures
            .get(3)
            .map(|m| m.as_str().trim().to_string())
            .unwrap_or_default();
        if target.is_empty() || target.starts_with('#') {
            continue;
        }
        if is_external_link_target(&target) {
            links.push(Link {
                link_type: LinkType::External,
                target,
                label,
                resolved_path: String::new(),
            });
            continue;
        }
        links.push(Link {
            link_type: LinkType::Internal,
            resolved_path: resolve_markdown_local_link_with_context(&target, ctx),
            target,
            label,
        });
    }

    links
}

pub fn resolve_internal_link(raw_path: &str) -> String {
    let mut clean = raw_path.trim().to_string();
    clean = clean.trim_start_matches('$').to_string();
    clean = split_once_any(&clean, ['#', '?']).to_string();
    clean = clean.strip_suffix(".md").unwrap_or(&clean).to_string();
    clean_path(&clean)
}

pub fn resolve_markdown_local_link(raw_path: &str, source_path: &str) -> String {
    resolve_markdown_local_link_with_context(
        raw_path,
        &ParseContext {
            source_rel_path: source_path.to_string(),
            notes_root_abs: String::new(),
        },
    )
}

pub fn resolve_markdown_local_link_with_context(raw_path: &str, ctx: &ParseContext) -> String {
    let mut clean = raw_path.trim().trim_matches(['<', '>']).to_string();
    clean = split_once_any(&clean, ['#', '?']).to_string();
    if clean.is_empty() {
        return String::new();
    }
    clean = expand_home_path(&clean);

    if Path::new(&clean).is_absolute() {
        if !ctx.notes_root_abs.is_empty() {
            return rel_if_under_root(&clean, &ctx.notes_root_abs).unwrap_or_default();
        }
        return clean_path(&clean);
    }

    let source_dir = source_dir(&ctx.source_rel_path);
    clean_path(&join_slash(&source_dir, &clean))
}

pub fn normalize_wiki_link_target(raw: &str) -> String {
    let mut target = raw.trim();
    if let Some((before, _)) = target.split_once('|') {
        target = before;
    }
    if let Some((before, _)) = target.split_once('#') {
        target = before;
    }
    target.trim().to_string()
}

pub fn is_external_link_target(target: &str) -> bool {
    let lower = target.trim().to_ascii_lowercase();
    lower.starts_with("http://") || lower.starts_with("https://") || lower.starts_with("mailto:")
}

fn capture_first(captures: &regex::Captures<'_>, indexes: &[usize]) -> String {
    indexes
        .iter()
        .find_map(|idx| {
            captures
                .get(*idx)
                .map(|m| m.as_str())
                .filter(|s| !s.is_empty())
        })
        .unwrap_or_default()
        .to_string()
}

fn expand_home_path(path: &str) -> String {
    if path == "~" {
        return home_dir().unwrap_or_else(|| path.to_string());
    }
    if let Some(rest) = path.strip_prefix("~/") {
        if let Some(home) = home_dir() {
            return join_slash(&home, rest);
        }
    }
    path.to_string()
}

fn home_dir() -> Option<String> {
    env::var("HOME").ok().filter(|home| !home.trim().is_empty())
}

fn rel_if_under_root(abs_target: &str, notes_root: &str) -> Option<String> {
    let root = clean_path(&expand_home_path(notes_root));
    let target = clean_path(&expand_home_path(abs_target));

    if target == root {
        return Some(".".to_string());
    }
    let prefix = format!("{root}/");
    target.strip_prefix(&prefix).map(|rel| clean_path(rel))
}

fn source_dir(path: &str) -> String {
    let path = path.replace('\\', "/");
    match path.rsplit_once('/') {
        Some((dir, _)) => dir.to_string(),
        None => String::new(),
    }
}

fn join_slash(base: &str, child: &str) -> String {
    if base.is_empty() {
        child.to_string()
    } else if child.is_empty() {
        base.to_string()
    } else {
        format!(
            "{}/{}",
            base.trim_end_matches('/'),
            child.trim_start_matches('/')
        )
    }
}

fn split_once_any(value: &str, chars: impl IntoIterator<Item = char>) -> &str {
    let idx = chars
        .into_iter()
        .filter_map(|ch| value.find(ch))
        .min()
        .unwrap_or(value.len());
    &value[..idx]
}

fn clean_path(path: &str) -> String {
    let path = path.replace('\\', "/");
    let absolute = path.starts_with('/');
    let mut stack: Vec<&str> = Vec::new();

    for part in path.split('/') {
        match part {
            "" | "." => {}
            ".." => {
                if let Some(last) = stack.last() {
                    if *last != ".." {
                        stack.pop();
                        continue;
                    }
                }
                if !absolute {
                    stack.push("..");
                }
            }
            _ => stack.push(part),
        }
    }

    let joined = stack.join("/");
    if absolute {
        if joined.is_empty() {
            "/".to_string()
        } else {
            format!("/{joined}")
        }
    } else if joined.is_empty() {
        ".".to_string()
    } else {
        joined
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn extracts_wiki_links() {
        let parsed = parse_links(
            r#"
# Hub

- [[benefits]]
- [[ez-pass|EZ Pass]]
- [[medical#Insurance]]
"#,
            "introspection/remember/hub.md",
        );

        assert_eq!(parsed.len(), 3, "{parsed:#?}");
        assert_eq!(parsed[0].target, "benefits");
        assert_eq!(parsed[0].label, "benefits");
        assert_eq!(parsed[0].link_type, LinkType::Internal);
        assert_eq!(parsed[1].target, "ez-pass");
        assert_eq!(parsed[1].label, "EZ Pass");
        assert_eq!(parsed[2].target, "medical");
    }

    #[test]
    fn extracts_standard_markdown_links() {
        let parsed = parse_links(
            r#"
[Local](child.md)
[Relative](../other/hub.md#section)
[Absolute](/Users/me/notes/absolute.md)
[External](https://example.com)
![Image](diagram.png)
"#,
            "areas/current/hub.md",
        );

        assert_eq!(parsed.len(), 4, "{parsed:#?}");
        assert_eq!(parsed[0].link_type, LinkType::Internal);
        assert_eq!(parsed[0].resolved_path, "areas/current/child.md");
        assert_eq!(parsed[1].resolved_path, "areas/other/hub.md");
        assert_eq!(parsed[2].resolved_path, "/Users/me/notes/absolute.md");
        assert_eq!(parsed[3].link_type, LinkType::External);
        assert_eq!(parsed[3].target, "https://example.com");
    }

    #[test]
    fn root_aware_absolute_links() {
        let home = env::var("HOME").unwrap_or_else(|_| "/Users/me".to_string());
        let notes_root = format!("{home}/notes");
        let abs_under_root = format!("{notes_root}/autodactyl/topic.md");
        let abs_outside_root = format!("{home}/elsewhere/topic.md");

        let content = format!(
            r#"
[UnderRoot]({abs_under_root}#part)
[TildeUnderRoot](~/notes/autodactyl/tilde.md?x=1)
[OutsideRoot]({abs_outside_root})
[Relative](../other.md)
"#
        );
        let parsed = parse_links_with_context(
            &content,
            &ParseContext {
                source_rel_path: "areas/current/hub.md".to_string(),
                notes_root_abs: notes_root,
            },
        );

        assert_eq!(parsed.len(), 4, "{parsed:#?}");
        assert_eq!(parsed[0].resolved_path, "autodactyl/topic.md");
        assert_eq!(parsed[1].resolved_path, "autodactyl/tilde.md");
        assert_eq!(parsed[2].resolved_path, "");
        assert_eq!(parsed[3].resolved_path, "areas/other.md");
    }

    #[test]
    fn resolves_custom_internal_links() {
        assert_eq!(resolve_internal_link("$foo/bar.md#part"), "foo/bar");
        assert_eq!(resolve_internal_link("foo/../bar.md?x=1"), "bar");
    }
}
