//! Snapshot-based task mutations. Hash/recheck plus rename is not external-editor CAS.
pub mod archive;
mod layout;
#[cfg(test)]
mod tests;
use super::*;
use sha2::{Digest, Sha256};
use std::{
    io::{self, Write},
    path::PathBuf,
};

const HEADER: &str = "<!-- mw-dashboard:v1 -->";

#[cfg(test)]
thread_local! {
    static HOOK: std::cell::RefCell<Option<Box<dyn FnMut(&str)>>> = const { std::cell::RefCell::new(None) };
}

fn checkpoint(stage: &str) {
    #[cfg(test)]
    HOOK.with(|hook| {
        if let Some(hook) = hook.borrow_mut().as_mut() {
            hook(stage);
        }
    });
    #[cfg(not(test))]
    let _ = stage;
}

pub(super) fn invalid(message: impl Into<String>) -> io::Error {
    io::Error::new(io::ErrorKind::InvalidData, message.into())
}

pub(super) fn revision(bytes: &[u8]) -> String {
    format!("sha256:{:x}", Sha256::digest(bytes))
}

fn valid_hex(value: &str, len: usize) -> bool {
    value.len() == len
        && value
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
}

fn valid_baseline_identity(b: &Baseline) -> bool {
    let valid_id = if let Some(uid) = b.task_id.strip_prefix("task:").filter(|s| valid_hex(s, 32)) {
        !uid.is_empty()
    } else if let Some(position) = b.task_id.strip_prefix(&format!("{}:", b.source_id)) {
        position.split_once(':').is_some_and(|(line, order)| {
            line.parse::<usize>()
                .is_ok_and(|n| n > 0 && n.to_string() == line)
                && order.parse::<usize>().is_ok_and(|n| n.to_string() == order)
        })
    } else {
        false
    };
    valid_id
        && !b.source_path.is_empty()
        && Path::new(&b.source_path)
            .components()
            .all(|c| matches!(c, std::path::Component::Normal(_)))
        && !b.source_id.is_empty()
        && !b.source_id.contains(['\r', '\n', '\0', ']'])
        && !b.text.is_empty()
        && !b.text.contains(['\r', '\n', '\0'])
        && b.source_revision
            .strip_prefix("sha256:")
            .is_some_and(|s| valid_hex(s, 64))
}

fn check_revision(content: &str, expected: &str) -> io::Result<()> {
    if !expected
        .strip_prefix("sha256:")
        .is_some_and(|s| valid_hex(s, 64))
    {
        return Err(invalid(
            "--expected-revision must be the sourceRevision from query todos or todos inspect; inspect again and review changes before retrying",
        ));
    }
    if revision(content.as_bytes()) != expected {
        return Err(invalid(
            "stale source revision; inspect/query the current task and review changes before retrying",
        ));
    }
    Ok(())
}

pub(super) fn mask_task_examples(content: &str) -> io::Result<String> {
    let prefix_lines =
        extract_frontmatter_prefix(content)?.map_or(0, |prefix| prefix.lines().count());
    let mut fence: Option<(u8, usize)> = None;
    Ok(content
        .split('\n')
        .enumerate()
        .map(|(index, line)| {
            let masked = || if line.ends_with('\r') { "\r" } else { "" }.to_string();
            if index < prefix_lines {
                return masked();
            }
            let trimmed = line.trim_start();
            let ch = trimmed.as_bytes().first().copied().unwrap_or_default();
            let count = trimmed.bytes().take_while(|b| *b == ch).count();
            let boundary = matches!(ch, b'`' | b'~') && count >= 3;
            if let Some((marker, size)) = fence {
                if boundary && ch == marker && count >= size && trimmed[count..].trim().is_empty() {
                    fence = None;
                }
                masked()
            } else if boundary {
                fence = Some((ch, count));
                masked()
            } else {
                line.to_string()
            }
        })
        .collect::<Vec<_>>()
        .join("\n"))
}

pub(super) fn identities(
    content: &str,
    tasks: &[ParsedTask],
) -> io::Result<BTreeMap<usize, String>> {
    let lines: Vec<_> = content.split('\n').collect();
    let mut out = BTreeMap::new();
    for (index, line) in lines.iter().enumerate() {
        if !line.contains("mw-task:") && !line.contains("<!-- mw-task") {
            continue;
        }
        let uid = line
            .trim()
            .strip_prefix("<!-- mw-task:v1:")
            .and_then(|s| s.strip_suffix(" -->"))
            .filter(|s| valid_hex(s, 32))
            .ok_or_else(|| {
                invalid(format!(
                    "malformed reserved task marker on line {}",
                    index + 1
                ))
            })?;
        let owner = tasks
            .iter()
            .rev()
            .find(|t| t.line <= index)
            .filter(|t| {
                let indent = line_indent(lines[t.line - 1]);
                line_indent(line) > indent
                    && lines[t.line..=index].iter().all(|l| {
                        l.trim().is_empty()
                            || (parse_heading(l.trim()).is_none()
                                && !source::checkbox_like(l)
                                && line_indent(l) > indent)
                    })
            })
            .ok_or_else(|| invalid(format!("orphan task marker on line {}", index + 1)))?;
        if out.insert(owner.line, uid.to_string()).is_some() {
            return Err(invalid("duplicate task markers in one block"));
        }
    }
    Ok(out)
}

struct Lock(PathBuf);
impl Lock {
    fn acquire(root: &Path) -> io::Result<Self> {
        if !root.is_dir() {
            return Err(invalid(format!(
                "notes root must already exist and be a directory: {}",
                root.display()
            )));
        }
        let dir = root.join(".mw");
        match fs::create_dir(&dir) {
            Ok(()) => {}
            Err(e) if e.kind() == io::ErrorKind::AlreadyExists && dir.is_dir() => {}
            Err(e) => return Err(e),
        }
        let path = dir.join("todos.lock");
        let mut file = fs::OpenOptions::new().write(true).create_new(true).open(&path)
            .map_err(|e| invalid(format!("cannot acquire {}: {e}; another task operation may be running; inspect the lock PID, never automatically remove it", path.display())))?;
        let guard = Self(path);
        writeln!(file, "pid={}", std::process::id())?;
        Ok(guard)
    }
}
impl Drop for Lock {
    fn drop(&mut self) {
        let _ = fs::remove_file(&self.0);
    }
}

#[derive(Clone)]
struct Snapshot {
    path: PathBuf,
    bytes: Option<Vec<u8>>,
}
impl Snapshot {
    fn read(path: &Path) -> io::Result<Self> {
        let bytes = match fs::symlink_metadata(path) {
            Ok(meta) => {
                if !meta.is_file() || meta.file_type().is_symlink() {
                    return Err(invalid(format!(
                        "refusing non-regular or symlink file: {}",
                        path.display()
                    )));
                }
                Some(fs::read(path)?)
            }
            Err(e) if e.kind() == io::ErrorKind::NotFound => None,
            Err(e) => return Err(e),
        };
        Ok(Self {
            path: path.to_path_buf(),
            bytes,
        })
    }
    fn recheck(&self) -> io::Result<()> {
        if Self::read(&self.path)?.bytes != self.bytes {
            return Err(invalid(format!(
                "file changed before commit: {}; no overwrite attempted",
                self.path.display()
            )));
        }
        Ok(())
    }
    fn prepare(&self, bytes: &[u8]) -> io::Result<tempfile::NamedTempFile> {
        let parent = self
            .path
            .parent()
            .ok_or_else(|| invalid("file has no parent"))?;
        let mut temp = tempfile::NamedTempFile::new_in(parent)?;
        if self.bytes.is_some() {
            temp.as_file()
                .set_permissions(fs::metadata(&self.path)?.permissions())?;
        }
        temp.write_all(bytes)?;
        temp.as_file().sync_all()?;
        Ok(temp)
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct Baseline {
    task_id: String,
    source_path: String,
    source_id: String,
    source_revision: String,
    done: bool,
    text: String,
}

fn encode(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}
fn decode(hex: &str) -> io::Result<Vec<u8>> {
    if hex.len() % 2 != 0 || !valid_hex(hex, hex.len()) {
        return Err(invalid("invalid dashboard baseline hex"));
    }
    (0..hex.len())
        .step_by(2)
        .map(|i| {
            u8::from_str_radix(&hex[i..i + 2], 16).map_err(|_| invalid("invalid baseline hex"))
        })
        .collect()
}

fn dashboard_rows(snapshot: &Snapshot, discard: bool) -> io::Result<Vec<(Baseline, bool)>> {
    let Some(bytes) = &snapshot.bytes else {
        return Ok(Vec::new());
    };
    let content = std::str::from_utf8(bytes).map_err(|_| invalid("dashboard is not UTF-8"))?;
    let prefix = extract_frontmatter_prefix(content)?;
    let body = &content[prefix.map_or(0, str::len)..];
    if discard {
        return Ok(Vec::new());
    }
    if body.lines().filter(|l| *l == HEADER).count() != 1 {
        return Err(invalid(
            "missing/unsupported dashboard version; use todos refresh --discard-dashboard-edits only after reviewing the dashboard",
        ));
    }
    layout::validate(body)?;
    let checkbox =
        Regex::new(r"^- \[([ xX])\] (.*) <!-- mw-dashboard:v1:([0-9a-f]+) --> \[\[([^\]]+)\]\]$")
            .unwrap();
    let mut rows = Vec::new();
    let mut seen = BTreeSet::new();
    let mut header_seen = false;
    let mut title_seen = false;
    let mut groups = 0;
    for line in body.lines() {
        if line.trim().is_empty() {
            continue;
        }
        if line == HEADER && !header_seen && !title_seen {
            header_seen = true;
            continue;
        }
        if header_seen && !title_seen && layout::is_marker(line) {
            continue;
        }
        if header_seen && line == "# Dashboard" && !title_seen {
            title_seen = true;
            continue;
        }
        if title_seen
            && groups < FOCUS_GROUPS.len()
            && line == format!("## {}", FOCUS_GROUPS[groups])
        {
            groups += 1;
            continue;
        }
        if title_seen && groups > 0 && line.starts_with("- [") {
            let c = checkbox.captures(line).ok_or_else(|| {
                invalid("malformed/unsupported dashboard row; dashboard preserved")
            })?;
            let baseline: Baseline = serde_json::from_slice(&decode(&c[3])?)
                .map_err(|e| invalid(format!("invalid dashboard baseline: {e}")))?;
            if !valid_baseline_identity(&baseline) || !seen.insert(baseline.task_id.clone()) {
                return Err(invalid("invalid or duplicate dashboard baseline identity"));
            }
            if c[2] != baseline.text || c[4] != baseline.source_id {
                return Err(invalid(
                    "dashboard text/link changed; only checkbox writeback is supported; dashboard preserved",
                ));
            }
            rows.push((baseline, c[1].eq_ignore_ascii_case("x")));
        } else {
            return Err(invalid(
                "unsupported dashboard body edit; only generated headings, blank lines, and checkbox rows are supported; use refresh --discard-dashboard-edits after reviewing body edits",
            ));
        }
    }
    if title_seen && groups != FOCUS_GROUPS.len() {
        return Err(invalid(
            "incomplete generated dashboard headings; body preserved",
        ));
    }
    Ok(rows)
}

fn pending(rows: &[(Baseline, bool)]) -> bool {
    rows.iter().any(|(b, done)| b.done != *done)
}

fn render(todos: &[TaskIndexTodo], dashboard: &Snapshot) -> io::Result<Vec<u8>> {
    let content = std::str::from_utf8(dashboard.bytes.as_deref().unwrap_or_default())
        .map_err(|_| invalid("dashboard is not UTF-8"))?;
    let prefix =
        extract_frontmatter_prefix(content)?.unwrap_or_else(|| default_dashboard_frontmatter());
    let separator = if prefix.ends_with('\n') { "" } else { "\n" };
    let mut out = format!("{HEADER}\n# Dashboard\n\n");
    for area in FOCUS_GROUPS {
        out.push_str(&format!("## {area}\n"));
        for todo in todos
            .iter()
            .filter(|t| t.todo_section == "Next" && t.area == *area)
        {
            if todo.text.contains(['\r', '\n', '\0'])
                || todo.text.contains("mw-dashboard:")
                || todo.text.contains("mw-dashboard-layout:")
                || todo.source_id.contains(['\r', '\n', '\0', ']'])
            {
                return Err(invalid(
                    "task text/source ID cannot be safely represented in dashboard protocol",
                ));
            }
            let baseline = Baseline {
                task_id: todo.id.clone(),
                source_path: todo.source_path.clone(),
                source_id: todo.source_id.clone(),
                source_revision: todo.source_revision.clone(),
                done: todo.done,
                text: todo.text.clone(),
            };
            if !valid_baseline_identity(&baseline) {
                return Err(invalid(
                    "task provenance cannot be represented in dashboard baseline",
                ));
            }
            let hex = encode(&serde_json::to_vec(&baseline).map_err(|e| invalid(e.to_string()))?);
            out.push_str(&format!(
                "- [{}] {} <!-- mw-dashboard:v1:{hex} --> [[{}]]\n",
                if todo.done { "x" } else { " " },
                todo.text,
                todo.source_id
            ));
        }
        out.push('\n');
    }
    let marker = layout::marker(&out);
    let out = out.replacen(HEADER, &format!("{HEADER}\n{marker}"), 1);
    Ok(format!("{prefix}{separator}{out}").into_bytes())
}

fn source_snapshot(root: &Path, rel: &str, original: &str) -> io::Result<Snapshot> {
    let snapshot = Snapshot::read(&root.join(rel))?;
    if snapshot.bytes.as_deref() != Some(original.as_bytes()) {
        return Err(invalid("source changed since scan; inspect again"));
    }
    Ok(snapshot)
}

fn reject_dashboard_source(
    root: &Path,
    dashboard: &Snapshot,
    sources: &BTreeMap<String, String>,
) -> io::Result<()> {
    let resolved = fs::canonicalize(&dashboard.path).ok();
    if sources.keys().any(|rel| {
        root.join(rel) == dashboard.path
            || (resolved.is_some() && fs::canonicalize(root.join(rel)).ok() == resolved)
    }) {
        return Err(invalid("dashboard must not also be an active task source"));
    }
    Ok(())
}

fn commit(
    source: Option<(&Snapshot, &[u8])>,
    dashboard: &Snapshot,
    projection: &[u8],
) -> io::Result<()> {
    if source.is_some_and(|(s, _)| {
        s.path == dashboard.path
            || (fs::canonicalize(&s.path).ok().is_some()
                && fs::canonicalize(&s.path).ok() == fs::canonicalize(&dashboard.path).ok())
    }) {
        return Err(invalid("dashboard must not also be a task source"));
    }
    let temp = dashboard.prepare(projection)?;
    let source_temp = source
        .map(|(snapshot, bytes)| snapshot.prepare(bytes))
        .transpose()?;
    checkpoint("before-commit");
    dashboard.recheck()?;
    if let Some((snapshot, _)) = source {
        snapshot.recheck()?;
        source_temp
            .ok_or_else(|| invalid("source preparation missing"))?
            .persist(&snapshot.path)
            .map_err(|e| e.error)?;
        checkpoint("source-committed");
        let projection_result = dashboard.recheck().and_then(|()| {
            temp.persist(&dashboard.path)
                .map(|_| ())
                .map_err(|e| e.error)
        });
        if let Err(e) = projection_result {
            return Err(invalid(format!(
                "source committed; dashboard projection NOT updated: {e}; blind retry is unsafe; inspect source and dashboard"
            )));
        }
    } else {
        dashboard.recheck()?;
        temp.persist(&dashboard.path).map_err(|e| e.error)?;
    }
    Ok(())
}

pub(super) fn project(
    root: &Path,
    path: &Path,
    sync: bool,
    discard: bool,
) -> io::Result<SyncStats> {
    let _lock = Lock::acquire(root)?;
    let dashboard = Snapshot::read(path)?;
    let rows = dashboard_rows(&dashboard, discard)?;
    if !sync && pending(&rows) {
        return Err(invalid(
            "pending dashboard checkbox edits; sync them or explicitly refresh --discard-dashboard-edits",
        ));
    }
    let (mut todos, mut stats, snapshots) = scan_task_snapshots(root)?;
    reject_dashboard_source(root, &dashboard, &snapshots)?;
    let mut changes = Vec::new();
    let mut paths = BTreeSet::new();
    for (b, done) in rows {
        if done == b.done {
            continue;
        }
        let task = todos
            .iter()
            .find(|t| t.id == b.task_id)
            .ok_or_else(|| invalid("edited dashboard task missing; dashboard preserved"))?;
        if task.source_path != b.source_path
            || task.source_id != b.source_id
            || task.source_revision != b.source_revision
            || task.text != b.text
            || task.done != b.done
        {
            return Err(invalid(
                "stale dashboard checkbox/provenance; source and dashboard preserved",
            ));
        }
        paths.insert(task.source_path.clone());
        changes.push((task.clone(), done));
    }
    if paths.len() > 1 {
        return Err(invalid(
            "pending dashboard changes span multiple source files; this bounded sync supports at most one modified source; split/review edits before retrying",
        ));
    }
    let source = if let Some(rel) = paths.first() {
        let original = &snapshots[rel];
        let snapshot = source_snapshot(root, rel, original)?;
        let mut lines: Vec<String> = original.split('\n').map(str::to_string).collect();
        for (task, done) in &changes {
            lines[task.line - 1] = update_checkbox_in_line(&lines[task.line - 1], *done);
        }
        let updated = lines.join("\n");
        let rev = revision(updated.as_bytes());
        for todo in &mut todos {
            if todo.source_path == *rel {
                todo.source_revision = rev.clone();
            }
            if let Some((_, done)) = changes.iter().find(|(t, _)| t.id == todo.id) {
                todo.done = *done;
            }
        }
        Some((snapshot, updated))
    } else {
        None
    };
    let projection = render(&todos, &dashboard)?;
    commit(
        source.as_ref().map(|(s, text)| (s, text.as_bytes())),
        &dashboard,
        &projection,
    )?;
    stats.source_writebacks = changes.len();
    stats.source_files_updated = usize::from(source.is_some());
    stats.synced_tasks = 0;
    stats.tasks_by_area.clear();
    for todo in todos.iter().filter(|t| t.todo_section == "Next") {
        stats.synced_tasks += 1;
        *stats.tasks_by_area.entry(todo.area.clone()).or_default() += 1;
    }
    Ok(stats)
}

pub fn refresh_dashboard_discarding_edits(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
) -> io::Result<SyncStats> {
    project(notes_dir.as_ref(), dashboard_path.as_ref(), false, true)
}

enum Edit<'a> {
    Toggle,
    Identify,
    Update(&'a TodoUpdateParams),
}

fn mutate(
    root: &Path,
    dashboard_path: &Path,
    ids: &[String],
    expected: &str,
    edit: Edit<'_>,
) -> io::Result<Vec<TaskIndexTodo>> {
    let _lock = Lock::acquire(root)?;
    if dashboard_path.as_os_str().is_empty() {
        return Err(invalid(
            "dashboard path is required for guarded mutation preflight",
        ));
    }
    let dashboard = Snapshot::read(dashboard_path)?;
    let (todos, _, snapshots) = scan_task_snapshots(root)?;
    checkpoint("after-scan");
    reject_dashboard_source(root, &dashboard, &snapshots)?;
    let mut seen = BTreeSet::new();
    let mut targets = Vec::new();
    for id in ids {
        if !seen.insert(id) {
            return Err(invalid("duplicate requested task ID"));
        }
        targets.push(
            todos
                .iter()
                .find(|t| t.id == *id)
                .cloned()
                .ok_or_else(|| invalid(format!("task not found: {id}")))?,
        );
    }
    let first = targets
        .first()
        .ok_or_else(|| invalid("at least one task ID is required"))?;
    let rel = first.source_path.clone();
    if targets.iter().any(|t| t.source_path != rel) {
        return Err(invalid(
            "one expected revision supports only one source file; cross-source bulk update rejected",
        ));
    }
    let original = &snapshots[&rel];
    check_revision(original, expected)?;
    let source = source_snapshot(root, &rel, original)?;
    if pending(&dashboard_rows(&dashboard, false)?) {
        return Err(invalid(
            "pending dashboard checkbox edits; sync or explicitly discard them before direct mutation",
        ));
    }
    if matches!(edit, Edit::Identify) && first.stable_id.is_some() {
        return Ok(targets);
    }
    let mut lines: Vec<String> = original.split('\n').map(str::to_string).collect();
    // Bottom-up edits keep the observed line coordinates valid.
    targets.sort_by_key(|t| std::cmp::Reverse(t.line));
    for task in &targets {
        let index = task.line - 1;
        match &edit {
            Edit::Toggle => lines[index] = update_checkbox_in_line(&lines[index], !task.done),
            Edit::Identify => {
                let mut random = [0; 16];
                getrandom::fill(&mut random)
                    .map_err(|e| invalid(format!("random identity failed: {e}")))?;
                let uid = encode(&random);
                if todos.iter().any(|t| t.stable_id.as_deref() == Some(&uid)) {
                    return Err(invalid("random identity collision; no writes"));
                }
                let indent: String = lines[index]
                    .chars()
                    .take_while(|c| matches!(c, ' ' | '\t'))
                    .collect();
                let cr = if lines[index].ends_with('\r') {
                    "\r"
                } else {
                    ""
                };
                let marker = format!("{indent}  <!-- mw-task:v1:{uid} -->{cr}");
                if index + 1 == lines.len() && !original.ends_with('\n') {
                    lines.push(marker);
                } else {
                    lines.insert(index + 1, marker);
                }
            }
            Edit::Update(params) => {
                if params.title.is_some() && targets.len() != 1 {
                    return Err(invalid("title edits require one task"));
                }
                let metadata_edit = [
                    &params.area,
                    &params.priority,
                    &params.energy,
                    &params.weight,
                    &params.due,
                    &params.start,
                    &params.estimate,
                    &params.metadata,
                ]
                .iter()
                .any(|v| v.is_some())
                    || !params.clear.is_empty();
                // Reject mixed prose rather than reconstructing it from recognized tokens.
                let indent = line_indent(&lines[index]);
                for line in lines.iter().skip(index + 1).take_while(|l| {
                    l.trim().is_empty()
                        || (line_indent(l) > indent && parse_heading(l.trim()).is_none())
                }) {
                    if metadata_edit
                        && (line.trim_start().starts_with("```")
                            || line.trim_start().starts_with("~~~")
                            || (is_task_metadata_line(line.trim()) && !pure_metadata(line)))
                    {
                        return Err(invalid(
                            "unsupported fenced or mixed metadata/prose child line; no files changed",
                        ));
                    }
                }
                lines = update_task_metadata_in_lines(lines, index, task, params)?;
            }
        }
    }
    let updated = lines.join("\n");
    // Parse the planned bytes with the same metadata and identity rules, without disk writes.
    let planned = planned_todos(&todos, &rel, &updated)?;
    let orders: BTreeSet<_> = targets.iter().map(|t| t.order).collect();
    let result = planned
        .iter()
        .filter(|t| t.source_path == rel && orders.contains(&t.order))
        .cloned()
        .collect();
    let projection = render(&planned, &dashboard)?;
    commit(Some((&source, updated.as_bytes())), &dashboard, &projection)?;
    Ok(result)
}

fn pure_metadata(line: &str) -> bool {
    let stripped = strip_metadata_bullet_prefix(line);
    let re = Regex::new(r"(?i)\b(?:area:\s*(?:Code|Action|Actions|Reading|Amusement|Music|Exercise|Love)|(?:priority:\s*)?p:?[1-5]|(?:energy:\s*|e:)(?:xsm|xs|x-small|small|s|medium|m|large|l|xl|x-large)|(?:w|weight):\s*[0-9]+(?:\.[0-9]+)?|(?:due|start):\s*\d{4}-\d{2}-\d{2}|(?:est|estimate):\s*[0-9]+)\b").unwrap();
    re.replace_all(&stripped, "").trim().is_empty()
}

fn planned_todos(
    todos: &[TaskIndexTodo],
    rel: &str,
    content: &str,
) -> io::Result<Vec<TaskIndexTodo>> {
    let masked = mask_task_examples(content)?;
    let parsed = extract_todo_tasks(&masked)?;
    let identities = identities(&masked, &parsed)?;
    let meta = source::metadata(content)?.unwrap_or_default();
    let area = resolve_area(
        meta.raw
            .get("task_area")
            .map(String::as_str)
            .unwrap_or_default(),
        "Action",
    );
    let mut out: Vec<_> = todos
        .iter()
        .filter(|t| t.source_path != rel)
        .cloned()
        .collect();
    for task in parsed {
        let mut todo = todos
            .iter()
            .find(|t| t.source_path == rel && t.order == task.order)
            .cloned()
            .ok_or_else(|| invalid("edit changed task structure; rejected"))?;
        todo.stable_id = identities.get(&task.line).cloned();
        todo.id = todo
            .stable_id
            .as_ref()
            .map(|id| format!("task:{id}"))
            .unwrap_or_else(|| format!("{}:{}:{}", todo.source_id, task.line, task.order));
        todo.identity_kind = if todo.stable_id.is_some() {
            "stable"
        } else {
            "positional"
        }
        .to_string();
        todo.source_revision = revision(content.as_bytes());
        todo.line = task.line;
        todo.done = task.done;
        (todo.area, todo.text) =
            resolve_task_area_and_text_with_metadata(&task.text, &task.meta, &area);
        todo.metadata = build_todo_metadata(
            &todo.text,
            &task.meta,
            &todo.area,
            &todo.todo_section,
            todo.done,
            &todo.metadata.default_priority,
            &todo.metadata.default_energy,
        );
        out.push(todo);
    }
    if out.len() != todos.len() {
        return Err(invalid("edit changed task count; rejected"));
    }
    out.sort_by(|a, b| {
        a.source_path
            .cmp(&b.source_path)
            .then(a.order.cmp(&b.order))
    });
    Ok(out)
}

pub fn identify_task_index_todo(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
    id: &str,
    expected_revision: &str,
) -> io::Result<TaskIndexTodo> {
    mutate(
        notes_dir.as_ref(),
        dashboard_path.as_ref(),
        &[id.to_string()],
        expected_revision,
        Edit::Identify,
    )?
    .pop()
    .ok_or_else(|| invalid("identified task missing"))
}

pub fn toggle_task_index_todo(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
    id: &str,
    expected_revision: &str,
) -> io::Result<(TaskIndexTodo, bool)> {
    let todo = mutate(
        notes_dir.as_ref(),
        dashboard_path.as_ref(),
        &[id.to_string()],
        expected_revision,
        Edit::Toggle,
    )?
    .pop()
    .ok_or_else(|| invalid("toggled task missing"))?;
    let done = todo.done;
    Ok((todo, done))
}

pub fn update_task_index_todos(
    notes_dir: impl AsRef<Path>,
    dashboard_path: impl AsRef<Path>,
    params: TodoUpdateParams,
) -> io::Result<Vec<TaskIndexTodo>> {
    for value in [
        &params.title,
        &params.area,
        &params.priority,
        &params.energy,
        &params.weight,
        &params.due,
        &params.start,
        &params.estimate,
        &params.metadata,
    ]
    .into_iter()
    .flatten()
    .chain(params.clear.iter())
    {
        if value.contains(['\n', '\r', '\0']) || value.contains("mw-task:") {
            return Err(invalid(
                "task text/metadata options must not contain LF, CR, NUL, or reserved task markers",
            ));
        }
    }
    mutate(
        notes_dir.as_ref(),
        dashboard_path.as_ref(),
        &params.ids,
        &params.expected_revision,
        Edit::Update(&params),
    )
}
