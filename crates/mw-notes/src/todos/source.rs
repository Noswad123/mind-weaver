//! Task authority comes only from actual top-level YAML, never scalar contents.
use super::*;
use serde_yaml::Value;
use std::sync::OnceLock;

pub(super) fn checkbox_like(line: &str) -> bool {
    static CHECKBOX: OnceLock<Regex> = OnceLock::new();
    CHECKBOX
        .get_or_init(|| Regex::new(r"^\s*(?:[-*+]|[0-9]+[.)])\s*\[[ xX]\](?:\s|$)").unwrap())
        .is_match(line)
}

pub(super) fn metadata(content: &str) -> std::io::Result<Option<crate::Metadata>> {
    metadata_for_archive(content, false)
}

pub(super) fn metadata_for_archive(
    content: &str,
    historical: bool,
) -> std::io::Result<Option<crate::Metadata>> {
    let Some(prefix) = extract_frontmatter_prefix(content)? else {
        return Ok(None);
    };
    let mut lines = prefix.split_inclusive('\n');
    let start = lines.next().unwrap().len();
    let end = prefix.len() - lines.next_back().unwrap().len();
    let yaml: Value = serde_yaml::from_str(&prefix[start..end])
        .map_err(|e| safety::invalid(format!("invalid task frontmatter: {e}")))?;
    let mapping = match &yaml {
        Value::Null => return Ok(Some(crate::Metadata::default())),
        Value::Mapping(mapping) => mapping,
        _ => {
            return Err(safety::invalid(
                "task frontmatter must be a top-level YAML mapping",
            ));
        }
    };
    let mut metadata = crate::Metadata::default();
    let active = match mapping.get("task_active").unwrap_or(&Value::Null) {
        Value::Null => false,
        Value::Bool(value) => *value,
        Value::String(value) => match value.trim().to_ascii_lowercase().as_str() {
            "true" | "yes" | "on" | "1" => true,
            "" | "false" | "no" | "off" | "0" => false,
            _ => {
                return Err(safety::invalid(
                    "unsupported task_active value; use true or false",
                ));
            }
        },
        Value::Number(value) if value.as_u64().is_some_and(|n| n <= 1) => value.as_u64() == Some(1),
        _ => {
            return Err(safety::invalid(
                "task_active must be a boolean or supported scalar",
            ));
        }
    };
    metadata
        .raw
        .insert("task_active".to_string(), active.to_string());
    if !active {
        return Ok(Some(metadata));
    }
    metadata.domains = match mapping.get("domains").unwrap_or(&Value::Null) {
        Value::Null => Vec::new(),
        Value::String(value) => crate::parse_list_like_value(value),
        Value::Sequence(values) => values
            .iter()
            .map(|value| {
                value
                    .as_str()
                    .map(str::to_string)
                    .ok_or_else(|| safety::invalid("task domains must contain only strings"))
            })
            .collect::<std::io::Result<_>>()?,
        _ => {
            return Err(safety::invalid(
                "task domains must be a string or a list of strings",
            ));
        }
    };
    if metadata.domains.iter().any(|domain| {
        (!historical && domain.eq_ignore_ascii_case("task-index"))
            || domain.eq_ignore_ascii_case("taskIndex")
    }) {
        return Err(safety::invalid(
            "legacy task domain is not supported; migrate task-index/taskIndex to task and Todo headings to Do Now, Inbox, Blocked before using task operations",
        ));
    }
    if !metadata.domains.iter().any(|domain| {
        domain.eq_ignore_ascii_case("task")
            || (historical && domain.eq_ignore_ascii_case("task-index"))
    }) {
        return Ok(Some(metadata));
    }
    for (key, value) in mapping {
        let Some(key) = key.as_str() else {
            continue;
        };
        if value.is_null() {
            continue;
        }
        if matches!(
            key,
            "id" | "title"
                | "task_scope"
                | "task_area"
                | "task_default_priority"
                | "task_default_energy"
        ) {
            let scalar = match value {
                Value::String(value) => value.clone(),
                Value::Number(value) => value.to_string(),
                Value::Bool(value) => value.to_string(),
                _ => {
                    return Err(safety::invalid(format!(
                        "task field {key} must be a plain scalar, not a collection or tagged value"
                    )));
                }
            };
            metadata.raw.insert(key.to_string(), scalar);
        }
    }
    Ok(Some(metadata))
}
