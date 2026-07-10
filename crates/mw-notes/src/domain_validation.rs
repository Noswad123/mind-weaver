use std::collections::{BTreeMap, BTreeSet};

use regex::Regex;

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Deserialize)]
struct DomainSpec {
    domain: String,
    #[serde(default)]
    meta: MetaSpec,
    #[serde(default)]
    sections: SectionsSpec,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Deserialize)]
struct MetaSpec {
    #[serde(default)]
    required: Vec<String>,
    #[serde(default)]
    rules: MetaRules,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Deserialize)]
struct MetaRules {
    #[serde(default)]
    domains_must_include: String,
    #[serde(default)]
    language_style: String,
    #[serde(default)]
    concepts_style: String,
    #[serde(default)]
    path_suffix: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Deserialize)]
struct SectionsSpec {
    #[serde(default)]
    required_any: Vec<String>,
    #[serde(default)]
    required_all: Vec<String>,
    #[serde(default)]
    recommended: Vec<String>,
    #[serde(default)]
    structure: BTreeMap<String, SectionRules>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Deserialize)]
struct SectionRules {
    min_bullets: Option<usize>,
    require_code_block: Option<bool>,
    min_links: Option<usize>,
    min_children: Option<usize>,
    #[serde(default)]
    required_children: Vec<String>,
    child_rules: Option<ChildRules>,
    code_block: Option<CodeBlockRules>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Deserialize)]
struct ChildRules {
    min_bullets: Option<usize>,
    min_links: Option<usize>,
    checkbox_bullets_only: Option<bool>,
    #[serde(default)]
    required_fields: Vec<String>,
    #[serde(default)]
    recommended_fields: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Deserialize)]
struct CodeBlockRules {
    require_language_tag: Option<bool>,
    default_language_from_meta: Option<bool>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize)]
pub struct DomainViolation {
    pub path: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub errors: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub warns: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct DomainValidationResult {
    pub domain: String,
    pub checked: usize,
    pub violations: Vec<DomainViolation>,
    pub has_errors: bool,
}

pub fn validate_domain_notes<I, P, C>(
    domain: &str,
    notes: I,
) -> Result<DomainValidationResult, String>
where
    I: IntoIterator<Item = (P, C)>,
    P: AsRef<str>,
    C: AsRef<str>,
{
    let spec = load_domain_spec(domain)?;
    let mut violations = Vec::new();
    let mut has_errors = false;
    let mut checked = 0;

    for (path, content) in notes {
        checked += 1;
        if let Some(violation) = validate_note(path.as_ref(), content.as_ref(), &spec) {
            if !violation.errors.is_empty() {
                has_errors = true;
            }
            violations.push(violation);
        }
    }

    Ok(DomainValidationResult {
        domain: spec.domain,
        checked,
        violations,
        has_errors,
    })
}

pub fn canonical_domain_name(domain: &str) -> Result<String, String> {
    load_domain_spec(domain).map(|spec| spec.domain)
}

fn validate_note(path: &str, content: &str, spec: &DomainSpec) -> Option<DomainViolation> {
    let mut violation = DomainViolation {
        path: path.to_string(),
        errors: Vec::new(),
        warns: Vec::new(),
    };

    let domains = read_domains(content);
    if !domain_included(&domains, &spec.domain) {
        return None;
    }

    let required_domain = spec.meta.rules.domains_must_include.trim();
    if !required_domain.is_empty() && !domain_included(&domains, required_domain) {
        violation
            .errors
            .push(format!("meta.domains must include {required_domain}"));
    }
    if !spec.meta.rules.path_suffix.is_empty() && !path.ends_with(&spec.meta.rules.path_suffix) {
        violation.errors.push(format!(
            "path must end with {}",
            spec.meta.rules.path_suffix
        ));
    }

    for required in &spec.meta.required {
        match required.as_str() {
            "id" => {
                let (id, ok) = crate::read_meta_id_from_content(content);
                if !ok || id.trim().is_empty() {
                    violation.errors.push("missing meta.id".to_string());
                }
            }
            "domains" => {
                if read_meta_string_list(content, "domains")
                    .filter(|values| !values.is_empty())
                    .is_none()
                {
                    violation.errors.push("missing meta.domains".to_string());
                }
            }
            "concepts" => match read_meta_string_list(content, "concepts") {
                Some(concepts) if !concepts.is_empty() => {
                    if spec.meta.rules.concepts_style == "uid"
                        && concepts.iter().any(|concept| concept.trim().is_empty())
                    {
                        violation
                            .errors
                            .push("meta.concepts contains empty uid".to_string());
                    }
                }
                _ => violation.errors.push("missing meta.concepts".to_string()),
            },
            "language" => match read_meta_string(content, "language") {
                Some(language) if !language.trim().is_empty() => {
                    if spec.meta.rules.language_style == "slug" && !is_slug(&language) {
                        violation.errors.push(
                            "meta.language must be a slug (lowercase letters/numbers/hyphen)"
                                .to_string(),
                        );
                    }
                }
                _ => violation.errors.push("missing meta.language".to_string()),
            },
            other => {
                if read_meta_raw(content, other)
                    .filter(|raw| !raw.trim().is_empty())
                    .is_none()
                {
                    violation.errors.push(format!("missing meta.{other}"));
                }
            }
        }
    }

    let sections = extract_all_sections(content);
    if !spec.sections.required_any.is_empty()
        && !spec
            .sections
            .required_any
            .iter()
            .any(|section| sections.contains(section))
    {
        violation.errors.push(format!(
            "missing one of required sections: {}",
            spec.sections.required_any.join(", ")
        ));
    }
    for section in &spec.sections.required_all {
        if !sections.contains(section) {
            violation
                .errors
                .push(format!("missing required section: {section}"));
        }
    }
    for section in &spec.sections.recommended {
        if !sections.contains(section) {
            violation
                .warns
                .push(format!("missing recommended section: {section}"));
        }
    }

    apply_structure_rules(&mut violation, content, spec);

    if violation.errors.is_empty() && violation.warns.is_empty() {
        None
    } else {
        Some(violation)
    }
}

fn apply_structure_rules(violation: &mut DomainViolation, content: &str, spec: &DomainSpec) {
    for (section_name, rules) in &spec.sections.structure {
        let Some(block) = extract_section_block(content, section_name) else {
            continue;
        };

        if let Some(min) = rules.min_bullets.filter(|min| *min > 0) {
            if count_bullets(&block) < min {
                violation.errors.push(format!(
                    "{section_name} must contain at least {min} bullet(s)"
                ));
            }
        }
        if let Some(min) = rules.min_links.filter(|min| *min > 0) {
            if count_links(&block) < min {
                violation.errors.push(format!(
                    "{section_name} must contain at least {min} link(s)"
                ));
            }
        }
        if rules.require_code_block == Some(true) && !has_code_block(&block) {
            violation.errors.push(format!(
                "{section_name} must contain at least one @code ... @end block"
            ));
        }
        if let Some(code_rules) = &rules.code_block {
            let blocks = find_code_blocks(&block);
            if !blocks.is_empty() || rules.require_code_block == Some(true) {
                if code_rules.require_language_tag == Some(true) {
                    if blocks.iter().any(|code| code.language.trim().is_empty()) {
                        violation.errors.push(format!(
                            "{section_name} contains a @code block missing a language tag (expected: @code <lang>)"
                        ));
                    }
                } else if code_rules.default_language_from_meta == Some(true) {
                    let language = read_meta_string(content, "language").unwrap_or_default();
                    if language.trim().is_empty()
                        && blocks.iter().any(|code| code.language.trim().is_empty())
                    {
                        violation.errors.push(format!(
                            "{section_name} has @code block(s) without language and meta.language is missing"
                        ));
                    }
                }
            }
        }

        let needs_children = !rules.required_children.is_empty()
            || rules.min_children.is_some()
            || rules.child_rules.is_some();
        if !needs_children {
            continue;
        }
        let children = extract_immediate_children_blocks(&block);
        for child_name in &rules.required_children {
            if !child_name.trim().is_empty()
                && !children.iter().any(|child| child.name == *child_name)
            {
                violation.errors.push(format!(
                    "{section_name} section missing required child: {child_name}"
                ));
            }
        }
        if let Some(min) = rules.min_children.filter(|min| *min > 0) {
            if children.len() < min {
                violation.errors.push(format!(
                    "{section_name} section must contain at least {min} child section(s) (** ... / ## ...)"
                ));
            }
        }
        let Some(child_rules) = &rules.child_rules else {
            continue;
        };
        if let Some(min) = child_rules.min_bullets.filter(|min| *min > 0) {
            for child in &children {
                if count_bullets(&child.block) < min {
                    violation.errors.push(format!(
                        "{section_name} child '{}' must contain at least {min} bullet(s)",
                        child.name
                    ));
                }
            }
        }
        if child_rules.checkbox_bullets_only == Some(true) {
            for child in &children {
                if !child_has_only_checkbox_bullets(&child.block) {
                    violation.errors.push(format!(
                        "{section_name} child '{}' must use markdown checkboxes for bullets",
                        child.name
                    ));
                }
            }
        }
        if !child_rules.required_fields.is_empty() || !child_rules.recommended_fields.is_empty() {
            for child in &children {
                let fields = parse_child_fields(&child.block);
                for field in &child_rules.required_fields {
                    if !field.trim().is_empty() && !fields.contains_key(field) {
                        violation.errors.push(format!(
                            "{section_name} child '{}' missing required field: {field}",
                            child.name
                        ));
                    }
                }
                for field in &child_rules.recommended_fields {
                    if !field.trim().is_empty() && !fields.contains_key(field) {
                        violation.warns.push(format!(
                            "{section_name} child '{}' missing recommended field: {field}",
                            child.name
                        ));
                    }
                }
            }
        }
    }
}

fn load_domain_spec(domain: &str) -> Result<DomainSpec, String> {
    let domain = domain.trim();
    let lower_camel = slug_to_lower_camel(domain);
    for candidate in [
        format!("{domain}.yaml"),
        format!("{domain}.yml"),
        format!("{lower_camel}.yaml"),
        format!("{lower_camel}.yml"),
    ] {
        if let Some(contents) = embedded_domain_schema(&candidate) {
            let spec: DomainSpec = serde_yaml::from_str(contents)
                .map_err(|err| format!("invalid domain schema {candidate}: {err}"))?;
            if spec.domain.trim().is_empty() {
                return Err(format!("domain schema {candidate} missing 'domain'"));
            }
            return Ok(spec);
        }
    }
    Err(format!("unknown domain schema: {domain}"))
}

fn embedded_domain_schema(name: &str) -> Option<&'static str> {
    match name {
        "abbreviation-index.yaml" | "abbreviationIndex.yaml" => Some(include_str!(
            "../../../legacy/go/internal/schema/templates/abbreviation-index.yaml"
        )),
        "dev-tool.yaml" | "devTool.yaml" => Some(include_str!(
            "../../../legacy/go/internal/schema/templates/dev-tool.yaml"
        )),
        "glossary.yaml" => Some(include_str!(
            "../../../legacy/go/internal/schema/templates/glossary.yaml"
        )),
        "programming-concept.yaml" | "programmingConcept.yaml" => Some(include_str!(
            "../../../legacy/go/internal/schema/templates/programming-concept.yaml"
        )),
        "programming-implementation.yaml" | "programmingImplementation.yaml" => Some(include_str!(
            "../../../legacy/go/internal/schema/templates/programming-implementation.yaml"
        )),
        "programming-language.yaml" | "programmingLanguage.yaml" => Some(include_str!(
            "../../../legacy/go/internal/schema/templates/programming-language.yaml"
        )),
        "recipe.yaml" => Some(include_str!(
            "../../../legacy/go/internal/schema/templates/recipe.yaml"
        )),
        "task-index.yaml" | "taskIndex.yaml" => Some(include_str!(
            "../../../legacy/go/internal/schema/templates/task-index.yaml"
        )),
        "vocabulary-index.yaml" | "vocabularyIndex.yaml" => Some(include_str!(
            "../../../legacy/go/internal/schema/templates/vocabulary-index.yaml"
        )),
        "writing-character.yaml" | "writingCharacter.yaml" => Some(include_str!(
            "../../../legacy/go/internal/schema/templates/writing-character.yaml"
        )),
        _ => None,
    }
}

fn read_domains(content: &str) -> Vec<String> {
    let mut out = Vec::new();
    let mut seen = BTreeSet::new();
    if let Some(values) = read_meta_string_list(content, "domains") {
        for domain in values {
            let domain = domain.trim();
            if !domain.is_empty() && seen.insert(domain.to_string()) {
                out.push(domain.to_string());
            }
        }
    }
    out
}

fn domain_included(domains: &[String], wanted: &str) -> bool {
    let wanted = wanted.trim();
    !wanted.is_empty() && domains.iter().any(|domain| domain.trim() == wanted)
}

fn meta_block(content: &str) -> Option<&str> {
    let re = Regex::new(r"(?s)^---\s*\n(.*?)\n---").expect("valid meta regex");
    re.captures(content)
        .and_then(|captures| captures.get(1).map(|matched| matched.as_str()))
}

fn read_meta_string(content: &str, key: &str) -> Option<String> {
    read_meta_raw(content, key).and_then(|raw| {
        let val = trim_quotes(strip_inline_comment(&raw).trim());
        if val.is_empty() { None } else { Some(val) }
    })
}

fn read_meta_string_list(content: &str, key: &str) -> Option<Vec<String>> {
    let raw = read_meta_raw(content, key)?;
    let raw = raw.trim();
    if raw.is_empty() {
        return None;
    }
    if raw.starts_with('[') && raw.ends_with(']') {
        let inner = raw[1..raw.len() - 1].trim();
        if inner.is_empty() {
            return Some(Vec::new());
        }
        return Some(
            split_csv_loose(inner)
                .into_iter()
                .map(|part| trim_quotes(part.trim()))
                .filter(|part| !part.is_empty())
                .collect(),
        );
    }
    let value = trim_quotes(raw);
    if value.is_empty() {
        None
    } else {
        Some(vec![value])
    }
}

fn read_meta_raw(content: &str, key: &str) -> Option<String> {
    let re = Regex::new(r"^\s*([a-zA-Z_][a-zA-Z0-9_-]*)\s*(?:=|:)\s*(.+?)\s*$")
        .expect("valid meta kv regex");
    for line in meta_block(content)?.lines() {
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') || line.starts_with("//") {
            continue;
        }
        let Some(captures) = re.captures(line) else {
            continue;
        };
        if captures.get(1).map(|m| m.as_str().trim()) == Some(key) {
            return captures
                .get(2)
                .map(|m| strip_inline_comment(m.as_str()).trim().to_string());
        }
    }
    None
}

fn strip_inline_comment(s: &str) -> &str {
    if let Some((before, _)) = s.split_once(" #") {
        return before;
    }
    if let Some((before, _)) = s.split_once(" //") {
        return before;
    }
    s
}

fn trim_quotes(s: &str) -> String {
    let s = s.trim();
    if s.len() >= 2
        && ((s.starts_with('\'') && s.ends_with('\'')) || (s.starts_with('"') && s.ends_with('"')))
    {
        s[1..s.len() - 1].to_string()
    } else {
        s.to_string()
    }
}

fn split_csv_loose(s: &str) -> Vec<String> {
    let mut parts = Vec::new();
    let mut buf = String::new();
    let mut in_single = false;
    let mut in_double = false;
    for ch in s.chars() {
        match ch {
            '\'' if !in_double => {
                in_single = !in_single;
                buf.push(ch);
            }
            '"' if !in_single => {
                in_double = !in_double;
                buf.push(ch);
            }
            ',' if !in_single && !in_double => {
                let part = buf.trim();
                if !part.is_empty() {
                    parts.push(part.to_string());
                }
                buf.clear();
            }
            _ => buf.push(ch),
        }
    }
    let part = buf.trim();
    if !part.is_empty() {
        parts.push(part.to_string());
    }
    parts
}

fn extract_all_sections(content: &str) -> BTreeSet<String> {
    let mut sections = BTreeSet::new();
    for line in content.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with('#') || trimmed.starts_with('*') {
            if let Some((_, name)) = parse_heading_line(trimmed) {
                sections.insert(name);
                sections.insert(trimmed.to_string());
            }
        }
    }
    sections
}

fn extract_section_block(content: &str, name: &str) -> Option<String> {
    let lines: Vec<&str> = content.lines().collect();
    let start = lines.iter().position(|line| {
        parse_heading_line(line.trim()).is_some_and(|(_, heading)| heading == name)
    })?;
    let start_level = parse_heading_line(lines[start].trim())?.0;
    let end = lines[start + 1..]
        .iter()
        .position(|line| {
            parse_heading_line(line.trim()).is_some_and(|(level, _)| level <= start_level)
        })
        .map(|idx| start + 1 + idx)
        .unwrap_or(lines.len());
    Some(lines[start..end].join("\n"))
}

fn parse_heading_line(trimmed: &str) -> Option<(usize, String)> {
    let marker = match trimmed.as_bytes().first().copied()? {
        b'#' => '#',
        b'*' => '*',
        _ => return None,
    };
    let level = trimmed.chars().take_while(|ch| *ch == marker).count();
    if level == 0 || trimmed.chars().nth(level) != Some(' ') {
        return None;
    }
    let name = trimmed[level + 1..].trim();
    if name.is_empty() {
        None
    } else {
        Some((level, name.to_string()))
    }
}

fn count_bullets(block: &str) -> usize {
    block
        .lines()
        .filter(|line| line.trim().starts_with("- "))
        .count()
}

fn count_links(block: &str) -> usize {
    Regex::new(r"https?://[^\s\}\]>]+")
        .expect("valid link regex")
        .find_iter(block)
        .count()
}

fn has_code_block(block: &str) -> bool {
    !find_code_blocks(block).is_empty()
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct CodeBlock {
    language: String,
}

fn find_code_blocks(block: &str) -> Vec<CodeBlock> {
    let re = Regex::new(r"(?ms)^\s*@code(?:\s+([^\s]+))?\s*$.*?^\s*@end\s*$")
        .expect("valid code block regex");
    re.captures_iter(block)
        .map(|captures| CodeBlock {
            language: captures
                .get(1)
                .map(|m| m.as_str().to_string())
                .unwrap_or_default(),
        })
        .collect()
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct ChildBlock {
    name: String,
    block: String,
}

fn extract_immediate_children_blocks(section_block: &str) -> Vec<ChildBlock> {
    let lines: Vec<&str> = section_block.lines().collect();
    let mut children = Vec::new();
    let mut current_name = String::new();
    let mut current_start: Option<usize> = None;
    let flush = |children: &mut Vec<ChildBlock>, name: &str, start: Option<usize>, end: usize| {
        if let Some(start) = start {
            if !name.trim().is_empty() {
                children.push(ChildBlock {
                    name: name.to_string(),
                    block: lines[start..end].join("\n"),
                });
            }
        }
    };
    for (idx, line) in lines.iter().enumerate() {
        if let Some((level, heading)) = parse_heading_line(line.trim()) {
            if level == 2 {
                flush(&mut children, &current_name, current_start, idx);
                current_name = heading;
                current_start = Some(idx);
            }
        }
    }
    flush(&mut children, &current_name, current_start, lines.len());
    children
}

fn child_has_only_checkbox_bullets(block: &str) -> bool {
    let re = Regex::new(r"^\s*-\s*\[[ xX]\]\s+.+$").expect("valid checkbox regex");
    block
        .lines()
        .filter(|line| line.trim().starts_with("- "))
        .all(|line| re.is_match(line.trim()))
}

fn parse_child_fields(child_block: &str) -> BTreeMap<String, String> {
    let re = Regex::new(r"^\s*-\s*([a-zA-Z_][a-zA-Z0-9_-]*)\s*:\s*(.+?)\s*$")
        .expect("valid child field regex");
    let mut fields = BTreeMap::new();
    for line in child_block.lines() {
        if let Some(captures) = re.captures(line.trim_end_matches('\r')) {
            let key = captures.get(1).map(|m| m.as_str().trim()).unwrap_or("");
            let value = captures.get(2).map(|m| m.as_str().trim()).unwrap_or("");
            if !key.is_empty() && !value.is_empty() {
                fields.insert(key.to_string(), value.to_string());
            }
        }
    }
    fields
}

fn is_slug(s: &str) -> bool {
    Regex::new(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
        .expect("valid slug regex")
        .is_match(s.trim())
}

fn slug_to_lower_camel(s: &str) -> String {
    let mut parts = s
        .trim()
        .split(|ch| ch == '-' || ch == '_' || ch == ' ')
        .filter(|part| !part.is_empty());
    let Some(first) = parts.next() else {
        return String::new();
    };
    let mut out = first.to_string();
    for part in parts {
        let mut chars = part.chars();
        if let Some(first) = chars.next() {
            out.push(first.to_ascii_uppercase());
            out.push_str(chars.as_str());
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validates_glossary_required_sections_and_links() {
        let content = r#"---
id: test-term
domains: [glossary]
---

# Test Term

## Definition

A term.

## Further Reading

No links here.
"#;

        let result =
            validate_domain_notes("glossary", [("biology/test-term.md", content)]).unwrap();

        assert_eq!(result.domain, "glossary");
        assert_eq!(result.checked, 1);
        assert!(result.has_errors);
        assert_eq!(result.violations.len(), 1);
        assert!(
            result.violations[0]
                .errors
                .iter()
                .any(|err| err == "Further Reading must contain at least 1 link(s)")
        );
    }

    #[test]
    fn skips_notes_outside_domain() {
        let content = r#"---
id: other
domains: [journal]
---

# Other
"#;

        let result = validate_domain_notes("glossary", [("other.md", content)]).unwrap();

        assert_eq!(result.checked, 1);
        assert!(result.violations.is_empty());
        assert!(!result.has_errors);
    }

    #[test]
    fn supports_lower_camel_schema_lookup() {
        assert_eq!(
            canonical_domain_name("abbreviationIndex").unwrap(),
            "abbreviation-index"
        );
    }
}
