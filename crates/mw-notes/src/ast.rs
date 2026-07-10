#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct AstNode {
    #[serde(rename = "type")]
    pub node_type: String,
    #[serde(skip_serializing_if = "is_zero")]
    pub level: usize,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub text: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub lang: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub code: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub children: Vec<AstNode>,
}

fn is_zero(value: &usize) -> bool {
    *value == 0
}

pub fn parse_ast(content: &str) -> AstNode {
    let mut root_children = Vec::new();
    let body = strip_frontmatter(content);
    parse_blocks(
        body.lines().collect::<Vec<_>>().as_slice(),
        1,
        &mut root_children,
    );
    AstNode {
        node_type: "root".to_string(),
        children: root_children,
        ..AstNode::default()
    }
}

pub fn strip_frontmatter(content: &str) -> &str {
    let Some(rest) = content.strip_prefix("---") else {
        return content;
    };
    let rest = rest.strip_prefix('\n').unwrap_or(rest);
    if let Some((_, body)) = rest.split_once("\n---") {
        body.strip_prefix('\n').unwrap_or(body)
    } else {
        content
    }
}

fn parse_blocks(lines: &[&str], min_heading_level: usize, out: &mut Vec<AstNode>) {
    let mut idx = 0;
    while idx < lines.len() {
        let raw = lines[idx].trim_end_matches('\r');
        let trimmed = raw.trim();
        if trimmed.is_empty() {
            idx += 1;
            continue;
        }

        if let Some(lang) = trimmed.strip_prefix("@code") {
            let lang = lang.trim().to_string();
            idx += 1;
            let mut code_lines = Vec::new();
            while idx < lines.len() {
                if lines[idx].trim() == "@end" {
                    idx += 1;
                    break;
                }
                code_lines.push(lines[idx]);
                idx += 1;
            }
            out.push(AstNode {
                node_type: "code".to_string(),
                lang,
                code: code_lines.join("\n").trim_end_matches('\n').to_string(),
                ..AstNode::default()
            });
            continue;
        }

        if let Some((level, title)) = parse_heading(trimmed) {
            if level < min_heading_level {
                break;
            }
            let start = idx + 1;
            let mut end = start;
            while end < lines.len() {
                if let Some((next_level, _)) = parse_heading(lines[end].trim()) {
                    if next_level <= level {
                        break;
                    }
                }
                end += 1;
            }
            let mut children = Vec::new();
            parse_blocks(&lines[start..end], level + 1, &mut children);
            out.push(AstNode {
                node_type: "heading".to_string(),
                level,
                text: title,
                children,
                ..AstNode::default()
            });
            idx = end;
            continue;
        }

        if let Some(text) = trimmed.strip_prefix("- ") {
            out.push(AstNode {
                node_type: "bullet".to_string(),
                text: text.trim().to_string(),
                ..AstNode::default()
            });
            idx += 1;
            continue;
        }

        out.push(AstNode {
            node_type: "paragraph".to_string(),
            text: raw.trim().to_string(),
            ..AstNode::default()
        });
        idx += 1;
    }
}

fn parse_heading(trimmed: &str) -> Option<(usize, String)> {
    let level = trimmed.chars().take_while(|ch| *ch == '#').count();
    if level == 0 || trimmed.chars().nth(level) != Some(' ') {
        return None;
    }
    let title = trimmed[level + 1..].trim();
    if title.is_empty() {
        None
    } else {
        Some((level, title.to_string()))
    }
}

pub fn find_heading<'a>(node: &'a AstNode, level: usize, text: &str) -> Option<&'a AstNode> {
    if node.node_type == "heading" && node.level == level && node.text.trim() == text {
        return Some(node);
    }
    node.children
        .iter()
        .find_map(|child| find_heading(child, level, text))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_commands_heading_with_children() {
        let ast = parse_ast(
            r#"---
id: tool
---

# Tool

# Commands

## run
- template: tool run
- description: Run it
"#,
        );
        let commands = find_heading(&ast, 1, "Commands").unwrap();
        assert_eq!(commands.children[0].node_type, "heading");
        assert_eq!(commands.children[0].text, "run");
    }
}
