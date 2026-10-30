//! Checked copy-first archive. Digests detect edits, not malicious forgery; recheck/rename is not CAS.
use super::*;
use serde::{Deserialize, Serialize};

const MAX_TASKS: usize = 256;
const MAX_SOURCE: usize = 8 * 1024 * 1024;
const MAX_LEDGER: usize = 64 * 1024 * 1024;
const MAX_RECEIPT: usize = 2 * 1024 * 1024;
const MAX_RECORDS: usize = 256;
const END: &[u8] = b"\n<!-- mw-archive-end:v1 -->\n";

/// The reserved top-level note ID owns an opaque document, regardless of body validity/version.
/// Do not search the body: ordinary notes may contain ledger examples.
pub(crate) fn is_managed_archive_document(content: &str) -> bool {
    let mut lines = content.lines();
    if lines.next() != Some("---") {
        return false;
    }
    for line in lines {
        if line == "---" {
            break;
        }
        if let Some(value) = line.strip_prefix("id:") {
            if serde_yaml::from_str::<String>(value)
                .ok()
                .is_some_and(|id| {
                    id.strip_prefix("mw-archive-")
                        .is_some_and(|suffix| valid_hex(suffix, 32))
                })
            {
                return true;
            }
        }
    }
    false
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ArchiveItem {
    pub task: ArchiveTaskContextV1,
    pub byte_start: usize,
    pub byte_length: usize,
    pub block_revision: String,
}

/// Frozen V1 wire context. Changes to live task schemas must not change this grammar.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ArchiveTaskContextV1 {
    pub id: String,
    #[serde(deserialize_with = "required_nullable_string")]
    pub stable_id: Option<String>,
    pub identity_kind: String,
    pub source_revision: String,
    pub source_id: String,
    pub source_path: String,
    pub note_title: String,
    pub task_scope: String,
    pub todo_section: String,
    pub area: String,
    pub text: String,
    pub done: bool,
    pub order: usize,
    pub line: usize,
    pub metadata: ArchiveTodoMetadataV1,
}

/// Frozen V1 metadata, including historical clock-derived display weight.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ArchiveTodoMetadataV1 {
    pub status: String,
    pub todo_section: String,
    pub area: String,
    pub priority: String,
    pub energy: String,
    pub weight_override: String,
    pub due: String,
    pub start: String,
    pub estimate: String,
    pub raw: String,
    pub effective_weight: f64,
    pub default_priority: String,
    pub default_energy: String,
}

impl From<TaskIndexTodo> for ArchiveTaskContextV1 {
    fn from(task: TaskIndexTodo) -> Self {
        Self {
            id: task.id,
            stable_id: task.stable_id,
            identity_kind: task.identity_kind,
            source_revision: task.source_revision,
            source_id: task.source_id,
            source_path: task.source_path,
            note_title: task.note_title,
            task_scope: task.task_scope,
            todo_section: task.todo_section,
            area: task.area,
            text: task.text,
            done: task.done,
            order: task.order,
            line: task.line,
            metadata: task.metadata.into(),
        }
    }
}

impl From<TodoMetadata> for ArchiveTodoMetadataV1 {
    fn from(metadata: TodoMetadata) -> Self {
        Self {
            status: metadata.status,
            todo_section: metadata.todo_section,
            area: metadata.area,
            priority: metadata.priority,
            energy: metadata.energy,
            weight_override: metadata.weight_override,
            due: metadata.due,
            start: metadata.start,
            estimate: metadata.estimate,
            raw: metadata.raw,
            effective_weight: metadata.effective_weight,
            default_priority: metadata.default_priority,
            default_energy: metadata.default_energy,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ArchivePlan {
    pub schema_version: String,
    pub kind: String,
    pub operation_id: String,
    pub notes_dir: String,
    pub dashboard_path: String,
    pub archive_path: String,
    pub source_path: String,
    pub source_id: String,
    pub source_revision: String,
    pub post_source_revision: String,
    pub dashboard_revision: String,
    pub archive_revision: String,
    pub task_count: usize,
    pub raw_byte_count: usize,
    pub items: Vec<ArchiveItem>,
}

fn same_plan_semantics(actual: &ArchivePlan, confirmed: &ArchivePlan) -> bool {
    let mut actual = actual.clone();
    let mut confirmed = confirmed.clone();
    // This derived display value depends on today's due/start multipliers, not source authority.
    for plan in [&mut actual, &mut confirmed] {
        for item in &mut plan.items {
            item.task.metadata.effective_weight = 0.0;
        }
    }
    actual == confirmed
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ArchiveInspection {
    pub schema_version: String,
    pub kind: String,
    pub plan: ArchivePlan,
    pub phase: String,
    pub source_revision: String,
    pub archive_revision: String,
    pub dashboard_revision: String,
    pub source_state: String,
    pub next_action: String,
    #[serde(deserialize_with = "required_nullable_string")]
    pub blocked_reason: Option<String>,
    pub projection_status: String,
}

fn required_nullable_string<'de, D: serde::Deserializer<'de>>(
    d: D,
) -> Result<Option<String>, D::Error> {
    Option::<String>::deserialize(d)
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ArchiveResult {
    pub schema_version: String,
    pub kind: String,
    pub operation_id: String,
    pub archive_path: String,
    pub source_path: String,
    pub phase: String,
    pub source_pruned: bool,
    pub projection_updated: bool,
    pub already_complete: bool,
    pub task_count: usize,
    pub raw_byte_count: usize,
    pub source_revision: String,
    pub archive_revision: String,
    pub dashboard_revision: String,
}

#[derive(Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct Receipt {
    plan: ArchivePlan,
    phase: String,
}
struct Record {
    receipt: Receipt,
    payload: Vec<u8>,
}
struct Ledger {
    prefix: Vec<u8>,
    records: Vec<Record>,
}

/// Migration must not strand an operation which still needs its original source bytes.
/// Keep ledger validation here so callers cannot accidentally accept weaker framing.
pub(super) fn ensure_completed_ledger(path: &Path) -> io::Result<()> {
    let ledger = Ledger::read(&checked(path)?)?;
    if ledger.records.is_empty() {
        return Err(invalid(
            "incomplete archive ledger; inspect before migration",
        ));
    }
    if ledger
        .records
        .iter()
        .any(|r| r.receipt.phase != "completed")
    {
        return Err(invalid(
            "incomplete archive operation; inspect/recover before migration or capture",
        ));
    }
    Ok(())
}

/// Managed history inside the notes root retains capture IDs after archival.
/// Removing/moving that history outside the root removes this deduplication evidence.
pub(super) fn archived_stable_ids(root: &Path) -> io::Result<BTreeSet<String>> {
    let mut ids = BTreeSet::new();
    for path in walk_markdown_files(root)? {
        let snapshot = checked(&path)?;
        let bytes = snapshot
            .bytes
            .as_deref()
            .ok_or_else(|| invalid("archive source disappeared"))?;
        let text = std::str::from_utf8(bytes).map_err(|_| invalid("notes source is not UTF-8"))?;
        if !is_managed_archive_document(text) {
            continue;
        }
        let ledger = Ledger::read(&snapshot)?;
        if ledger
            .records
            .iter()
            .any(|r| r.receipt.phase != "completed")
        {
            return Err(invalid(
                "incomplete archive operation; inspect/recover before capture",
            ));
        }
        for record in ledger.records {
            for item in record.receipt.plan.items {
                if let Some(id) = item.task.stable_id {
                    ids.insert(id);
                }
            }
        }
    }
    Ok(ids)
}

fn nonce() -> io::Result<String> {
    let mut bytes = [0; 16];
    getrandom::fill(&mut bytes).map_err(|e| invalid(format!("random operation identity: {e}")))?;
    Ok(encode(&bytes))
}
fn utf8(path: &Path) -> io::Result<String> {
    path.to_str()
        .map(str::to_string)
        .ok_or_else(|| invalid("path must be literal UTF-8"))
}
fn destination(path: &Path) -> io::Result<PathBuf> {
    let name = path
        .file_name()
        .ok_or_else(|| invalid("destination needs a final filename"))?;
    let parent = path
        .parent()
        .filter(|p| !p.as_os_str().is_empty())
        .unwrap_or(Path::new("."));
    let result = fs::canonicalize(parent)?.join(name);
    checked(&result)?;
    Ok(result)
}
fn checked(path: &Path) -> io::Result<Snapshot> {
    let snapshot = Snapshot::read(path)?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::MetadataExt;
        if snapshot.bytes.is_some() && fs::metadata(path)?.nlink() > 1 {
            return Err(invalid(format!(
                "hardlinked file refused: {}",
                path.display()
            )));
        }
    }
    Ok(snapshot)
}
fn state(s: &Snapshot) -> String {
    s.bytes
        .as_deref()
        .map(revision)
        .unwrap_or_else(|| "absent".into())
}
fn clean(dashboard: &Snapshot) -> io::Result<()> {
    if pending(&dashboard_rows(dashboard, false)?) {
        return Err(invalid("pending dashboard checkbox edits; archive refused"));
    }
    Ok(())
}
fn validate_plan(p: &ArchivePlan) -> io::Result<()> {
    let hash = |s: &str| s.strip_prefix("sha256:").is_some_and(|s| valid_hex(s, 64));
    if p.schema_version != "1"
        || p.kind != "task-archive-plan"
        || !valid_hex(&p.operation_id, 32)
        || p.task_count == 0
        || p.task_count > MAX_TASKS
        || p.items.len() != p.task_count
        || p.raw_byte_count > MAX_SOURCE
        || !hash(&p.source_revision)
        || !hash(&p.post_source_revision)
        || p.source_revision == p.post_source_revision
        || !(p.dashboard_revision == "absent" || hash(&p.dashboard_revision))
        || !(p.archive_revision == "absent" || hash(&p.archive_revision))
        || !Path::new(&p.notes_dir).is_absolute()
        || !Path::new(&p.dashboard_path).is_absolute()
        || !Path::new(&p.archive_path).is_absolute()
        || p.source_path.is_empty()
        || !Path::new(&p.source_path)
            .components()
            .all(|c| matches!(c, std::path::Component::Normal(_)))
    {
        return Err(invalid("invalid/unsupported archive plan"));
    }
    let mut end = 0;
    let mut total = 0usize;
    let mut ids = BTreeSet::new();
    for item in &p.items {
        let next = item
            .byte_start
            .checked_add(item.byte_length)
            .ok_or_else(|| invalid("span overflow"))?;
        if item.byte_start < end
            || item.byte_length == 0
            || next > MAX_SOURCE
            || !hash(&item.block_revision)
            || !item.task.done
            || !item.task.metadata.effective_weight.is_finite()
            || item.task.source_path != p.source_path
            || item.task.source_id != p.source_id
            || item.task.source_revision != p.source_revision
            || !ids.insert(&item.task.id)
        {
            return Err(invalid("inconsistent archive selection/spans"));
        }
        end = next;
        total = total
            .checked_add(item.byte_length)
            .ok_or_else(|| invalid("payload overflow"))?;
    }
    if total != p.raw_byte_count {
        return Err(invalid("payload byte count mismatch"));
    }
    Ok(())
}

impl Ledger {
    fn read(s: &Snapshot) -> io::Result<Self> {
        let Some(bytes) = &s.bytes else {
            return Ok(Self {
                prefix: Vec::new(),
                records: Vec::new(),
            });
        };
        if bytes.len() > MAX_LEDGER {
            return Err(invalid("ledger exceeds 64 MiB limit"));
        }
        let text = std::str::from_utf8(bytes).map_err(|_| invalid("ledger not UTF-8"))?;
        extract_frontmatter_prefix(text)?.ok_or_else(|| invalid("not a managed archive ledger"))?;
        let head = "---\nid: mw-archive-";
        let tail = "\ntask_active: false\n---\n<!-- mw-archive-ledger:v1 -->\n";
        let prefix_len = head.len() + 32 + tail.len();
        if bytes.len() < prefix_len
            || !text.starts_with(head)
            || !text
                .get(head.len()..head.len() + 32)
                .is_some_and(|s| valid_hex(s, 32))
            || text.get(head.len() + 32..prefix_len) != Some(tail)
        {
            return Err(invalid(
                "not a managed archive ledger; human notes cannot be imported",
            ));
        }
        let mut ledger = Self {
            prefix: bytes[..prefix_len].to_vec(),
            records: Vec::new(),
        };
        let mut pos = prefix_len;
        let mut ops = BTreeSet::new();
        while pos < bytes.len() {
            if ledger.records.len() >= MAX_RECORDS {
                return Err(invalid("ledger exceeds 256 operation limit"));
            }
            let rest = &bytes[pos..];
            let line_end = rest
                .iter()
                .position(|b| *b == b'\n')
                .ok_or_else(|| invalid("incomplete receipt"))?;
            let line =
                std::str::from_utf8(&rest[..line_end]).map_err(|_| invalid("receipt UTF-8"))?;
            let hex = line
                .strip_prefix("<!-- mw-archive-receipt:v1:")
                .and_then(|s| s.strip_suffix(" -->"))
                .ok_or_else(|| invalid("invalid receipt frame"))?;
            if hex.len() > MAX_RECEIPT * 2 {
                return Err(invalid("receipt exceeds 2 MiB JSON limit"));
            }
            let receipt: Receipt = serde_json::from_slice(&decode(hex)?)
                .map_err(|e| invalid(format!("invalid receipt: {e}")))?;
            validate_plan(&receipt.plan)?;
            let previous_revision = if ledger.records.is_empty() {
                "absent".into()
            } else {
                revision(&ledger.bytes()?)
            };
            // Completed history is portable; decoding is not authorization to resume an operation.
            // inspect/recover separately enforce the selected receipt's original filesystem bindings.
            if receipt.plan.archive_revision != previous_revision
                || ledger
                    .records
                    .last()
                    .is_some_and(|r| r.receipt.phase != "completed")
            {
                return Err(invalid(
                    "inconsistent ledger history revision or incomplete earlier operation",
                ));
            }
            if !matches!(
                receipt.phase.as_str(),
                "archiveCopied" | "sourcePruned" | "completed"
            ) || !ops.insert(receipt.plan.operation_id.clone())
            {
                return Err(invalid("invalid/duplicate operation receipt"));
            }
            pos += line_end + 1;
            let end = pos
                .checked_add(receipt.plan.raw_byte_count)
                .ok_or_else(|| invalid("payload overflow"))?;
            let payload = bytes
                .get(pos..end)
                .ok_or_else(|| invalid("incomplete payload"))?
                .to_vec();
            std::str::from_utf8(&payload).map_err(|_| invalid("payload not UTF-8"))?;
            let mut offset = 0;
            for item in &receipt.plan.items {
                let block = &payload[offset..offset + item.byte_length];
                if revision(block) != item.block_revision || std::str::from_utf8(block).is_err() {
                    return Err(invalid("archive payload integrity failure"));
                }
                offset += item.byte_length;
            }
            if bytes.get(end..end + END.len()) != Some(END) {
                return Err(invalid("incomplete payload trailer"));
            }
            pos = end + END.len();
            ledger.records.push(Record { receipt, payload });
        }
        if ledger.records.is_empty() {
            return Err(invalid(
                "incomplete ledger: managed prefix has no operation receipt",
            ));
        }
        if ledger.bytes()? != *bytes {
            return Err(invalid("unsupported ledger edits/noncanonical receipt"));
        }
        Ok(ledger)
    }
    fn bytes(&self) -> io::Result<Vec<u8>> {
        if self.records.len() > MAX_RECORDS {
            return Err(invalid("ledger exceeds 256 operation limit"));
        }
        let mut out = self.prefix.clone();
        for record in &self.records {
            let json = serde_json::to_vec(&record.receipt).map_err(|e| invalid(e.to_string()))?;
            if json.len() > MAX_RECEIPT {
                return Err(invalid("receipt exceeds 2 MiB JSON limit"));
            }
            out.extend_from_slice(
                format!("<!-- mw-archive-receipt:v1:{} -->\n", encode(&json)).as_bytes(),
            );
            out.extend_from_slice(&record.payload);
            out.extend_from_slice(END);
        }
        if out.len() > MAX_LEDGER {
            return Err(invalid("ledger exceeds 64 MiB limit"));
        }
        Ok(out)
    }
}

fn span(content: &str, task: &TaskIndexTodo) -> io::Result<(usize, usize)> {
    let lines: Vec<_> = content.split_inclusive('\n').collect();
    let index = task
        .line
        .checked_sub(1)
        .ok_or_else(|| invalid("invalid task line"))?;
    let line = lines
        .get(index)
        .ok_or_else(|| invalid("task outside source"))?;
    let start = lines[..index].iter().map(|s| s.len()).sum();
    let indent = line_indent(line);
    let mut end = start + line.len();
    let mut offset = end;
    let mut fence: Option<(u8, usize)> = None;
    for line in &lines[index + 1..] {
        let trimmed = line.trim();
        let ch = trimmed.as_bytes().first().copied().unwrap_or_default();
        let count = trimmed.bytes().take_while(|b| *b == ch).count();
        if let Some((marker, size)) = fence {
            if ch == marker && count >= size && trimmed[count..].trim().is_empty() {
                if line_indent(line) <= indent {
                    return Err(invalid(
                        "ambiguous owned fence closure; closing fence must remain indented",
                    ));
                }
                fence = None;
            }
            offset += line.len();
            end = offset;
            continue;
        }
        if trimmed.is_empty() {
            offset += line.len();
            continue;
        }
        if line_indent(line) <= indent {
            if parse_heading(trimmed).is_none() && !source::checkbox_like(line) && offset == end {
                return Err(invalid(
                    "unsupported lazy task continuation; indent child prose or separate unrelated blocks",
                ));
            }
            break;
        }
        if parse_heading(trimmed).is_some() {
            return Err(invalid(
                "unfenced indented heading is a shared-parser boundary, not an owned archive child",
            ));
        }
        if matches!(ch, b'`' | b'~') && count >= 3 {
            fence = Some((ch, count));
        }
        offset += line.len();
        end = offset;
    }
    if fence.is_some() {
        return Err(invalid("unclosed owned task fence"));
    }
    Ok((start, end - start))
}

struct Prepared {
    plan: ArchivePlan,
    source: Snapshot,
    dashboard: Snapshot,
    archive: Snapshot,
    post: Vec<u8>,
    payload: Vec<u8>,
    projection: Vec<u8>,
}

fn archive_scan(
    root: &Path,
    receipt: Option<&ArchivePlan>,
    replacement: Option<(&str, &str)>,
    post: bool,
) -> io::Result<(Vec<TaskIndexTodo>, SyncStats, BTreeMap<String, String>)> {
    let Some(p) = receipt else {
        return scan_task_snapshots_with(root, replacement);
    };
    let mut replacements = BTreeMap::new();
    let snapshot;
    let content = if let Some((path, content)) = replacement {
        if path != p.source_path {
            return Err(invalid("recovery replacement is not receipt-bound"));
        }
        content
    } else {
        snapshot = checked(&root.join(&p.source_path))?;
        std::str::from_utf8(snapshot.bytes.as_deref().unwrap_or_default())
            .map_err(|_| invalid("recovery source is not UTF-8"))?
    };
    if content.len() > MAX_SOURCE {
        return Err(invalid("source exceeds 8 MiB limit"));
    }
    replacements.insert(p.source_path.clone(), content.to_string());
    scan_task_snapshots_for_archive(
        root,
        &replacements,
        Some((
            &p.source_path,
            if post {
                &p.post_source_revision
            } else {
                &p.source_revision
            },
        )),
    )
}

fn archive_projection(
    todos: &[TaskIndexTodo],
    dashboard: &Snapshot,
    recovery: bool,
) -> io::Result<Vec<u8>> {
    if !recovery {
        return render(todos, dashboard);
    }
    // Keep historical context intact for receipt/survivor proofs. Only projection selects Next
    // as today's Do Now; source bytes and the frozen receipt are never migrated here.
    let mut projected = todos.to_vec();
    for task in &mut projected {
        if task.todo_section == "Next" {
            task.todo_section = "Do Now".into();
        }
    }
    render(&projected, dashboard)
}

fn prove_survivors(
    todos: &[TaskIndexTodo],
    survivors: &[TaskIndexTodo],
    rel: &str,
    original: &str,
    post: &[u8],
    items: &[ArchiveItem],
) -> io::Result<()> {
    let mut offsets = vec![0];
    for (index, byte) in original.bytes().enumerate() {
        if byte == b'\n' {
            offsets.push(index + 1);
        }
    }
    let removals: Vec<_> = items
        .iter()
        .map(|item| {
            let end = item.byte_start + item.byte_length;
            let lines = original.as_bytes()[item.byte_start..end]
                .iter()
                .filter(|b| **b == b'\n')
                .count();
            (end, lines)
        })
        .collect();
    let post_revision = revision(post);
    let mut expected = Vec::new();
    for task in todos {
        let mut task = task.clone();
        if task.source_path == rel {
            let start = *offsets
                .get(task.line - 1)
                .ok_or_else(|| invalid("survivor line outside source"))?;
            let owner = items
                .iter()
                .find(|i| start >= i.byte_start && start < i.byte_start + i.byte_length);
            if let Some(selected) = items.iter().find(|i| i.task.id == task.id) {
                if owner != Some(selected) || start != selected.byte_start {
                    return Err(invalid(
                        "selected task does not own exactly its planned interval",
                    ));
                }
                continue;
            }
            if owner.is_some() {
                return Err(invalid(
                    "archive interval would remove an unselected task identity",
                ));
            }
            let (removed_lines, removed_tasks) = removals
                .iter()
                .filter(|(end, _)| *end <= start)
                .fold((0, 0), |(lines, tasks), (_, count)| {
                    (lines + count, tasks + 1)
                });
            task.line -= removed_lines;
            task.order -= removed_tasks;
            if task.stable_id.is_none() {
                task.id = format!("{}:{}:{}", task.source_id, task.line, task.order);
            }
            task.source_revision = post_revision.clone();
        }
        task.metadata.effective_weight = 0.0;
        expected.push(task);
    }
    let mut actual = survivors.to_vec();
    for task in &mut actual {
        task.metadata.effective_weight = 0.0;
    }
    if expected != actual {
        return Err(invalid(
            "archive prune changed unselected task identities, coordinates, or context",
        ));
    }
    Ok(())
}
fn prepare(
    root: &Path,
    dashboard_path: &Path,
    archive_path: &Path,
    ids: &[String],
    expected: &str,
    op: &str,
    recovery: Option<&ArchivePlan>,
) -> io::Result<Prepared> {
    if ids.is_empty() || ids.len() > MAX_TASKS || !valid_hex(op, 32) {
        return Err(invalid(
            "archive requires 1..256 explicit IDs and a valid operation ID",
        ));
    }
    let root = fs::canonicalize(root)?;
    let dashboard_path = destination(dashboard_path)?;
    let archive_path = destination(archive_path)?;
    if dashboard_path == archive_path {
        return Err(invalid("archive aliases dashboard"));
    }
    let dashboard = checked(&dashboard_path)?;
    clean(&dashboard)?;
    let archive = checked(&archive_path)?;
    let ledger = Ledger::read(&archive)?;
    if recovery.is_none() && ledger.records.len() >= MAX_RECORDS {
        return Err(invalid(
            "ledger exceeds 256 operation limit; select a new archive artifact",
        ));
    }
    if recovery.is_none()
        && let Some(record) = ledger
            .records
            .iter()
            .find(|r| r.receipt.plan.operation_id == op)
    {
        return Err(partial(
            &record.receipt.plan,
            &record.receipt.phase,
            invalid("operation already exists; inspect/recover explicitly, do not retry commit"),
        ));
    }
    if recovery.is_none()
        && ledger
            .records
            .iter()
            .any(|r| r.receipt.phase != "completed")
    {
        return Err(invalid(
            "archive has an incomplete operation; inspect/recover it explicitly before appending",
        ));
    }
    let (todos, _, sources) = archive_scan(&root, recovery, None, false)?;
    reject_dashboard_source(&root, &dashboard, &sources)?;
    for rel in sources.keys() {
        let path = fs::canonicalize(root.join(rel))?;
        checked(&path)?;
        if path == archive_path {
            return Err(invalid("archive must not be an active source"));
        }
    }
    let mut seen = BTreeSet::new();
    let mut selected = Vec::new();
    for id in ids {
        if !seen.insert(id) {
            return Err(invalid("duplicate requested task ID"));
        }
        let t = todos
            .iter()
            .find(|t| &t.id == id)
            .ok_or_else(|| invalid(format!("task not found: {id}")))?;
        if !t.done {
            return Err(invalid(
                "only explicitly selected completed tasks may be archived",
            ));
        }
        selected.push(t.clone());
    }
    selected.sort_by_key(|t| t.line);
    let first = &selected[0];
    if selected.iter().any(|t| t.source_path != first.source_path) {
        return Err(invalid("archive selection must belong to one source"));
    }
    let original = &sources[&first.source_path];
    if original.len() > MAX_SOURCE {
        return Err(invalid("source exceeds 8 MiB limit"));
    }
    check_revision(original, expected)?;
    let source = source_snapshot(&root, &first.source_path, original)?;
    if fs::canonicalize(&source.path)? != source.path {
        return Err(invalid("source contains a symlink directory alias"));
    }
    let mut items = Vec::new();
    let mut payload = Vec::new();
    let mut post = Vec::new();
    let mut cursor = 0;
    for task in selected.clone() {
        let (start, length) = span(original, &task)?;
        if start < cursor {
            return Err(invalid("overlapping task ownership"));
        }
        post.extend_from_slice(&original.as_bytes()[cursor..start]);
        let block = &original.as_bytes()[start..start + length];
        payload.extend_from_slice(block);
        items.push(ArchiveItem {
            task: task.into(),
            byte_start: start,
            byte_length: length,
            block_revision: revision(block),
        });
        cursor = start + length;
    }
    post.extend_from_slice(&original.as_bytes()[cursor..]);
    let survivors = archive_scan(
        &root,
        recovery,
        Some((
            &first.source_path,
            std::str::from_utf8(&post).map_err(|_| invalid("invalid prune UTF-8"))?,
        )),
        true,
    )?
    .0;
    prove_survivors(
        &todos,
        &survivors,
        &first.source_path,
        original,
        &post,
        &items,
    )?;
    let projection = archive_projection(&survivors, &dashboard, recovery.is_some())?;
    let plan = ArchivePlan {
        schema_version: "1".into(),
        kind: "task-archive-plan".into(),
        operation_id: op.into(),
        notes_dir: utf8(&root)?,
        dashboard_path: utf8(&dashboard_path)?,
        archive_path: utf8(&archive_path)?,
        source_path: first.source_path.clone(),
        source_id: first.source_id.clone(),
        source_revision: expected.into(),
        post_source_revision: revision(&post),
        dashboard_revision: state(&dashboard),
        archive_revision: state(&archive),
        task_count: items.len(),
        raw_byte_count: payload.len(),
        items,
    };
    validate_plan(&plan)?;
    let receipt = Receipt {
        plan: plan.clone(),
        phase: "archiveCopied".into(),
    };
    if serde_json::to_vec(&receipt)
        .map_err(|e| invalid(e.to_string()))?
        .len()
        > MAX_RECEIPT
    {
        return Err(invalid("planned receipt exceeds 2 MiB JSON limit"));
    }
    Ok(Prepared {
        plan,
        source,
        dashboard,
        archive,
        post,
        payload,
        projection,
    })
}

pub fn plan_archive(
    root: impl AsRef<Path>,
    dashboard: impl AsRef<Path>,
    archive: impl AsRef<Path>,
    ids: &[String],
    expected: &str,
) -> io::Result<ArchivePlan> {
    Ok(prepare(
        root.as_ref(),
        dashboard.as_ref(),
        archive.as_ref(),
        ids,
        expected,
        &nonce()?,
        None,
    )?
    .plan)
}

struct DestinationLock(PathBuf);
impl Drop for DestinationLock {
    fn drop(&mut self) {
        let _ = fs::remove_file(&self.0);
    }
}
fn locks(root: &Path, archive: &Path) -> io::Result<(Lock, DestinationLock)> {
    let path = archive.with_file_name(format!(
        ".{}.mw-archive.lock",
        archive.file_name().unwrap().to_string_lossy()
    ));
    let acquire = || -> io::Result<DestinationLock> {
        let mut file = fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&path)
            .map_err(|e| {
                invalid(format!(
                    "cannot acquire archive lock {}: {e}",
                    path.display()
                ))
            })?;
        let guard = DestinationLock(path.clone());
        writeln!(file, "pid={}", std::process::id())?;
        Ok(guard)
    };
    if root.join(".mw/todos.lock") < path {
        let a = Lock::acquire(root)?;
        Ok((a, acquire()?))
    } else {
        let b = acquire()?;
        Ok((Lock::acquire(root)?, b))
    }
}
fn configured(root: &Path, dashboard: &Path, p: &ArchivePlan) -> io::Result<()> {
    validate_plan(p)?;
    if utf8(&fs::canonicalize(root)?)? != p.notes_dir
        || utf8(&destination(dashboard)?)? != p.dashboard_path
        || utf8(&destination(Path::new(&p.archive_path))?)? != p.archive_path
    {
        return Err(invalid(
            "archive receipt/plan does not match configured canonical notes root, dashboard, or destination",
        ));
    }
    Ok(())
}
fn publish(snapshot: &Snapshot, bytes: &[u8]) -> io::Result<Snapshot> {
    let temp = snapshot.prepare(bytes)?;
    checked(&snapshot.path)?;
    snapshot.recheck()?;
    temp.persist(&snapshot.path).map_err(|e| e.error)?;
    let next = checked(&snapshot.path)?;
    if next.bytes.as_deref() != Some(bytes) {
        return Err(invalid("published bytes changed externally"));
    }
    Ok(next)
}
fn phase(
    ledger: &mut Ledger,
    index: usize,
    phase: &str,
    archive: &Snapshot,
) -> io::Result<Snapshot> {
    checkpoint("archive-before-phase-update");
    ledger.records[index].receipt.phase = phase.into();
    publish(archive, &ledger.bytes()?)
}
fn partial(p: &ArchivePlan, phase: &str, e: io::Error) -> io::Error {
    let remaining = match phase {
        "archiveCopied" => "source prune, dashboard projection and completion NOT confirmed",
        "sourcePruned" => {
            "source prune recorded; dashboard projection and completion NOT confirmed"
        }
        _ => "completed phase last confirmed; final observations NOT confirmed",
    };
    invalid(format!(
        "operationId={} archivePath={} durablePhase={phase}: {e}; {remaining}; no blind retry, inspect then explicitly recover",
        p.operation_id, p.archive_path
    ))
}

pub fn commit_archive(
    root: impl AsRef<Path>,
    dashboard: impl AsRef<Path>,
    plan: &ArchivePlan,
) -> io::Result<ArchiveResult> {
    let root = root.as_ref();
    let dashboard = dashboard.as_ref();
    configured(root, dashboard, plan)?;
    let _locks = locks(Path::new(&plan.notes_dir), Path::new(&plan.archive_path))?;
    let ids: Vec<_> = plan.items.iter().map(|i| i.task.id.clone()).collect();
    let p = prepare(
        root,
        dashboard,
        Path::new(&plan.archive_path),
        &ids,
        &plan.source_revision,
        &plan.operation_id,
        None,
    )?;
    if !same_plan_semantics(&p.plan, plan) {
        return Err(invalid(
            "confirmed archive plan is stale or modified; no files replaced",
        ));
    }
    let mut ledger = Ledger::read(&p.archive)?;
    if ledger.prefix.is_empty() {
        ledger.prefix = format!(
            "---\nid: mw-archive-{}\ntask_active: false\n---\n<!-- mw-archive-ledger:v1 -->\n",
            nonce()?
        )
        .into_bytes();
    }
    let index = ledger.records.len();
    ledger.records.push(Record {
        receipt: Receipt {
            plan: plan.clone(),
            phase: "archiveCopied".into(),
        },
        payload: p.payload,
    });
    let archive_bytes = ledger.bytes()?;
    // All three replacements are prepared before publishing the first copy.
    let archive_temp = p.archive.prepare(&archive_bytes)?;
    let source_temp = p.source.prepare(&p.post)?;
    let projection_temp = p.dashboard.prepare(&p.projection)?;
    checkpoint("archive-before-copy");
    checked(&p.archive.path)?;
    checked(&p.source.path)?;
    checked(&p.dashboard.path)?;
    p.archive.recheck()?;
    p.source.recheck()?;
    p.dashboard.recheck()?;
    archive_temp.persist(&p.archive.path).map_err(|e| e.error)?;
    let mut durable = "archiveCopied";
    let result = (|| {
        checkpoint("archive-copy-published");
        let mut archive = checked(&p.archive.path)?;
        if archive.bytes.as_deref() != Some(&archive_bytes) {
            return Err(invalid("archive changed after copy"));
        }
        Ledger::read(&archive)?;
        checkpoint("archive-pre-prune");
        checked(&archive.path)?;
        checked(&p.dashboard.path)?;
        archive.recheck()?;
        checked(&p.source.path)?;
        p.source.recheck()?;
        p.dashboard.recheck()?;
        source_temp.persist(&p.source.path).map_err(|e| e.error)?;
        checkpoint("archive-source-pruned");
        let source = checked(&p.source.path)?;
        if source.bytes.as_deref() != Some(&p.post) {
            return Err(invalid("source changed after prune"));
        }
        archive = phase(&mut ledger, index, "sourcePruned", &archive)?;
        durable = "sourcePruned";
        checkpoint("archive-before-projection");
        checked(&p.dashboard.path)?;
        source.recheck()?;
        archive.recheck()?;
        p.dashboard.recheck()?;
        projection_temp
            .persist(&p.dashboard.path)
            .map_err(|e| e.error)?;
        checkpoint("archive-projection-published");
        source.recheck()?;
        let projected = checked(&p.dashboard.path)?;
        if projected.bytes.as_deref() != Some(&p.projection) {
            return Err(invalid("dashboard changed after projection"));
        }
        archive = phase(&mut ledger, index, "completed", &archive)?;
        durable = "completed";
        Ok(result(plan, &archive, &projected, true, true, false))
    })();
    result.map_err(|e| partial(plan, durable, e))
}

fn result(
    p: &ArchivePlan,
    archive: &Snapshot,
    dashboard: &Snapshot,
    pruned: bool,
    updated: bool,
    already: bool,
) -> ArchiveResult {
    ArchiveResult {
        schema_version: "1".into(),
        kind: "task-archive-result".into(),
        operation_id: p.operation_id.clone(),
        archive_path: p.archive_path.clone(),
        source_path: p.source_path.clone(),
        phase: "completed".into(),
        source_pruned: pruned,
        projection_updated: updated,
        already_complete: already,
        task_count: p.task_count,
        raw_byte_count: p.raw_byte_count,
        source_revision: p.post_source_revision.clone(),
        archive_revision: state(archive),
        dashboard_revision: state(dashboard),
    }
}

pub fn inspect_archive(
    root: impl AsRef<Path>,
    dashboard: impl AsRef<Path>,
    archive_path: impl AsRef<Path>,
    operation: &str,
) -> io::Result<ArchiveInspection> {
    if !valid_hex(operation, 32) {
        return Err(invalid("operation ID must be 32 lowercase hex characters"));
    }
    let archive = checked(&destination(archive_path.as_ref())?)?;
    let ledger = Ledger::read(&archive)?;
    let record = ledger
        .records
        .iter()
        .find(|r| r.receipt.plan.operation_id == operation)
        .ok_or_else(|| invalid("operation receipt not found"))?;
    let p = &record.receipt.plan;
    configured(root.as_ref(), dashboard.as_ref(), p)?;
    if utf8(&archive.path)? != p.archive_path {
        return Err(invalid("receipt destination mismatch"));
    }
    let source_path = Path::new(&p.notes_dir).join(&p.source_path);
    if source_path.exists() && fs::canonicalize(&source_path)? != source_path {
        return Err(invalid("source path identity changed"));
    }
    let source = checked(&source_path)?;
    let dashboard = checked(Path::new(&p.dashboard_path))?;
    let rev = state(&source);
    let source_state = if rev == p.source_revision {
        "pre"
    } else if rev == p.post_source_revision {
        "post"
    } else if rev == "absent" {
        "missing"
    } else {
        "changed"
    };
    let mut blocked = None;
    let mut projection_status = "blocked";
    let mut next = "none";
    if let Err(e) = clean(&dashboard) {
        blocked = Some(e.to_string());
    } else if source_state == "post" {
        match archive_scan(Path::new(&p.notes_dir), Some(p), None, true).and_then(
            |(todos, _, sources)| {
                reject_dashboard_source(Path::new(&p.notes_dir), &dashboard, &sources)?;
                archive_projection(&todos, &dashboard, true)
            },
        ) {
            Ok(bytes) => {
                projection_status = if dashboard.bytes.as_deref() == Some(&bytes) {
                    "current"
                } else {
                    "stale"
                };
                if record.receipt.phase != "completed" || projection_status == "stale" {
                    next = "refreshOnly";
                }
            }
            Err(e) => blocked = Some(e.to_string()),
        }
    } else if source_state == "pre" && record.receipt.phase == "archiveCopied" {
        match rebuild(p, &archive, &ledger) {
            Ok(_) => {
                next = "resumePrune";
                projection_status = "stale";
            }
            Err(e) => blocked = Some(e.to_string()),
        }
    } else {
        blocked = Some("source changed, missing, or restored to pre-state after pruning; preserve evidence, old operation cannot prune".into());
    }
    Ok(ArchiveInspection {
        schema_version: "1".into(),
        kind: "task-archive-inspection".into(),
        plan: p.clone(),
        phase: record.receipt.phase.clone(),
        source_revision: rev,
        archive_revision: state(&archive),
        dashboard_revision: state(&dashboard),
        source_state: source_state.into(),
        next_action: next.into(),
        blocked_reason: blocked,
        projection_status: projection_status.into(),
    })
}

fn rebuild(p: &ArchivePlan, archive: &Snapshot, ledger: &Ledger) -> io::Result<Prepared> {
    let ids: Vec<_> = p.items.iter().map(|i| i.task.id.clone()).collect();
    let mut prepared = prepare(
        Path::new(&p.notes_dir),
        Path::new(&p.dashboard_path),
        Path::new(&p.archive_path),
        &ids,
        &p.source_revision,
        &p.operation_id,
        Some(p),
    )?;
    prepared.plan.archive_revision = p.archive_revision.clone();
    prepared.plan.dashboard_revision = p.dashboard_revision.clone();
    if !same_plan_semantics(&prepared.plan, p) {
        return Err(invalid(
            "original immutable archive plan no longer matches source",
        ));
    }
    let record = ledger
        .records
        .iter()
        .find(|r| r.receipt.plan.operation_id == p.operation_id)
        .ok_or_else(|| invalid("missing receipt"))?;
    if prepared.payload != record.payload {
        return Err(invalid(
            "original raw payload no longer matches verified archive copy",
        ));
    }
    prepared.archive = archive.clone();
    Ok(prepared)
}

pub fn recover_archive(
    root: impl AsRef<Path>,
    dashboard: impl AsRef<Path>,
    observed: &ArchiveInspection,
) -> io::Result<ArchiveResult> {
    if observed.schema_version != "1" || observed.kind != "task-archive-inspection" {
        return Err(invalid("unsupported inspection version/kind"));
    }
    let p = &observed.plan;
    configured(root.as_ref(), dashboard.as_ref(), p)?;
    let _locks = locks(Path::new(&p.notes_dir), Path::new(&p.archive_path))?;
    let current = inspect_archive(root, dashboard, &p.archive_path, &p.operation_id)?;
    if current != *observed {
        return Err(invalid(
            "stale/modified inspection; no automatic adoption of newer observations",
        ));
    }
    if let Some(reason) = &current.blocked_reason {
        return Err(invalid(format!("recovery blocked: {reason}")));
    }
    checkpoint("archive-recovery-after-inspect");
    let mut archive = checked(Path::new(&p.archive_path))?;
    let observed_source = checked(&Path::new(&p.notes_dir).join(&p.source_path))?;
    let mut dashboard = checked(Path::new(&p.dashboard_path))?;
    if state(&archive) != current.archive_revision
        || state(&observed_source) != current.source_revision
        || state(&dashboard) != current.dashboard_revision
    {
        return Err(partial(
            p,
            &current.phase,
            invalid(
                "inspection observation changed after locked verification; no automatic adoption",
            ),
        ));
    }
    let mut ledger = Ledger::read(&archive)?;
    let index = ledger
        .records
        .iter()
        .position(|r| r.receipt.plan.operation_id == p.operation_id)
        .ok_or_else(|| invalid("operation disappeared after inspection"))?;
    if current.next_action == "none" {
        return Ok(result(p, &archive, &dashboard, false, false, true));
    }
    let mut durable = current.phase.as_str();
    let action = (|| {
        let mut pruned = false;
        let (source, projection, projection_temp) = if current.source_state == "pre" {
            let prepared = rebuild(p, &archive, &ledger)?;
            let source_temp = prepared.source.prepare(&prepared.post)?;
            let projection_temp = dashboard.prepare(&prepared.projection)?;
            checked(&prepared.source.path)?;
            archive.recheck()?;
            dashboard.recheck()?;
            prepared.source.recheck()?;
            source_temp
                .persist(&prepared.source.path)
                .map_err(|e| e.error)?;
            pruned = true;
            checkpoint("archive-recovery-source-pruned");
            let source = checked(&prepared.source.path)?;
            if source.bytes.as_deref() != Some(&prepared.post) {
                return Err(invalid("source changed after recovery prune"));
            }
            (source, prepared.projection, projection_temp)
        } else {
            // Never resolve the old IDs in the post-state: positional IDs may now name survivors.
            let source = checked(&Path::new(&p.notes_dir).join(&p.source_path))?;
            let todos = archive_scan(Path::new(&p.notes_dir), Some(p), None, true)?.0;
            let projection = archive_projection(&todos, &dashboard, true)?;
            let temp = dashboard.prepare(&projection)?;
            (source, projection, temp)
        };
        if state(&source) != p.post_source_revision {
            return Err(invalid("source no longer equals recorded post-state"));
        }
        if ledger.records[index].receipt.phase == "archiveCopied" {
            archive = phase(&mut ledger, index, "sourcePruned", &archive)?;
            durable = "sourcePruned";
        }
        source.recheck()?;
        archive.recheck()?;
        dashboard.recheck()?;
        let updated = dashboard.bytes.as_deref() != Some(&projection);
        if updated {
            checked(&dashboard.path)?;
            dashboard.recheck()?;
            projection_temp
                .persist(&dashboard.path)
                .map_err(|e| e.error)?;
            dashboard = checked(&dashboard.path)?;
            if dashboard.bytes.as_deref() != Some(&projection) {
                return Err(invalid("projection changed after recovery publication"));
            }
        }
        source.recheck()?;
        dashboard.recheck()?;
        if ledger.records[index].receipt.phase != "completed" {
            archive = phase(&mut ledger, index, "completed", &archive)?;
            durable = "completed";
        }
        Ok(result(p, &archive, &dashboard, pruned, updated, false))
    })();
    action.map_err(|e| partial(p, durable, e))
}

#[cfg(test)]
mod tests;

#[cfg(test)]
#[path = "archive/historical_tests.rs"]
mod historical_tests;
