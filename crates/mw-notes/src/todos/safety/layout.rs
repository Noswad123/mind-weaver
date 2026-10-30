//! Accidental-edit integrity, not authentication. Checkbox state is intentionally mutable.
use super::*;

const PREFIX: &str = "<!-- mw-dashboard-layout:v1:sha256:";

pub(super) fn is_marker(line: &str) -> bool {
    line.strip_prefix(PREFIX)
        .and_then(|s| s.strip_suffix(" -->"))
        .is_some_and(|s| valid_hex(s, 64))
}

fn digest(body: &str) -> String {
    let canonical = body
        .lines()
        .filter(|line| !line.trim().is_empty() && !is_marker(line))
        .map(|line| {
            if line.starts_with("- [x] ") || line.starts_with("- [X] ") {
                format!("- [ ] {}", &line[6..])
            } else {
                line.to_string()
            }
        })
        .collect::<Vec<_>>()
        .join("\n");
    revision(canonical.as_bytes())
}

pub(super) fn marker(body: &str) -> String {
    format!("<!-- mw-dashboard-layout:v1:{} -->", digest(body))
}

pub(super) fn validate(body: &str) -> io::Result<()> {
    let lines: Vec<_> = body
        .lines()
        .filter(|line| !line.trim().is_empty())
        .collect();
    if lines == [HEADER] {
        return Ok(());
    }
    if lines.first() != Some(&HEADER)
        || !lines.get(1).is_some_and(|line| is_marker(line))
        || lines
            .iter()
            .filter(|line| line.starts_with("<!-- mw-dashboard-layout"))
            .count()
            != 1
        || lines[1] != marker(body)
    {
        return Err(invalid(
            "unsupported dashboard body edit: missing/invalid layout integrity marker or changed row membership/order/area/metadata; review and use refresh --discard-dashboard-edits to regenerate; no source writes",
        ));
    }
    Ok(())
}
