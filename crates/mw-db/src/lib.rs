use std::{
    fs,
    path::Path,
    sync::atomic::{AtomicU64, Ordering},
    time::{SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result, anyhow, bail};
use mw_core::{EMBEDDED_COMMAND_SCHEMA_PATH, EMBEDDED_SCHEMA_PATH, config::Config};
use mw_notes::{LinkType, ParsedNote};
use rusqlite::{Connection, params};

const EMBEDDED_NOTE_SCHEMA: &str = include_str!("../schema.sql");
const EMBEDDED_COMMAND_SCHEMA: &str = include_str!("../command-schema.sql");
const SYNC_STATE_LAST_SERVER_CURSOR_KEY: &str = "last_server_cursor";
static OP_COUNTER: AtomicU64 = AtomicU64::new(0);

pub struct NoteDb {
    conn: Connection,
}

pub struct CommandDb {
    conn: Connection,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DbInitReport {
    pub notes_db_path: String,
    pub commands_db_path: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RegisteredNoteId {
    pub note_id: Option<i64>,
    pub uid: String,
    pub path: String,
    pub is_hub: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RegistryConflict {
    pub note_id: Option<i64>,
    pub uid: Option<String>,
    pub path: String,
    pub is_hub: bool,
    pub reason: String,
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RegistryConflictRecord {
    #[serde(rename = "noteID")]
    pub note_id: Option<i64>,
    pub uid: Option<String>,
    pub path: String,
    pub is_hub: bool,
    pub reason: String,
    pub detected_at: String,
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RegistryEntryRecord {
    #[serde(rename = "noteID")]
    pub note_id: i64,
    pub uid: String,
    pub path: String,
    pub is_hub: bool,
    pub updated_at: String,
}

#[derive(Debug, Clone, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RecipeRecord {
    pub id: i64,
    #[serde(rename = "noteID")]
    pub note_id: i64,
    pub name: String,
    pub path: String,
    pub serving_size: String,
    pub prep_time: String,
    pub cooking_time: String,
    pub meal: String,
    pub instructions: String,
    #[serde(rename = "payloadJSON")]
    pub payload_json: String,
    pub updated_at: String,
}

#[derive(Debug, Clone, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct IngredientRecord {
    pub id: i64,
    pub name: String,
    pub ingredient_type: String,
    pub notes: String,
    pub recipe_count: i64,
    pub mention_count: i64,
    pub created_at: String,
    pub updated_at: String,
}

#[derive(Debug, Clone, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct IngredientMentionRecord {
    pub id: i64,
    #[serde(rename = "noteID")]
    pub note_id: i64,
    #[serde(rename = "recipeID")]
    pub recipe_id: i64,
    pub recipe_name: String,
    pub path: String,
    pub raw_text: String,
    pub raw_name: String,
    pub quantity_text: String,
    pub quantity_number: Option<f64>,
    pub unit_raw: String,
    #[serde(rename = "canonicalIngredientID")]
    pub canonical_ingredient_id: Option<i64>,
    pub canonical_name: String,
    pub line_number: Option<i64>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GraphResult {
    pub nodes: Vec<GraphNode>,
    pub edges: Vec<GraphEdge>,
    pub meta: GraphMeta,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GraphMeta {
    pub seed_count: usize,
    pub depth: i64,
    pub search: String,
    pub domain: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GraphNode {
    pub id: String,
    #[serde(rename = "noteID")]
    pub note_id: i64,
    pub uid: String,
    pub label: String,
    pub title: String,
    pub path: String,
    pub kind: String,
    pub tags: Vec<String>,
    pub domains: Vec<String>,
    pub matched: bool,
    pub unknown: bool,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GraphEdge {
    pub id: String,
    pub source: String,
    pub target: String,
    pub kind: String,
    pub label: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
struct GraphNodeRow {
    id: i64,
    uid: String,
    title: String,
    path: String,
    tags: Vec<String>,
    domains: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
struct GraphEdgeRow {
    source_id: i64,
    target_id: i64,
    label: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SyncOutboxRecord {
    pub id: i64,
    pub op_id: String,
    pub idempotency_key: String,
    pub entity_type: String,
    pub entity_key: String,
    pub op_type: String,
    pub payload: String,
    pub payload_hash: String,
    pub base_version: i64,
    pub status: String,
    pub attempt_count: i64,
    pub last_error: String,
    pub created_at: String,
    pub updated_at: String,
    pub acked_at: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SyncDiagnostics {
    pub local_cursor: String,
    pub pending_outbox_count: i64,
    pub pending_outbox_retried_count: i64,
    pub pending_outbox_max_attempt_count: i64,
    pub pending_outbox_oldest_created_at: String,
    pub pending_outbox_latest_failure: String,
    pub acked_outbox_count: i64,
    pub total_conflict_count: i64,
    pub unresolved_conflict_count: i64,
    pub oldest_unresolved_conflict_at: String,
    pub sync_entity_version_count: i64,
    pub synced_todo_count: i64,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct NoteRecord {
    pub id: i64,
    pub uid: String,
    pub path: String,
    pub title: String,
    pub content: String,
    pub tags: Vec<String>,
    pub domains: Vec<String>,
    pub links: Vec<mw_notes::Link>,
    pub updated_at: String,
}

impl NoteDb {
    pub fn open(db_path: impl AsRef<Path>, schema_path: Option<&str>) -> Result<Self> {
        ensure_parent_dir(db_path.as_ref())?;
        let conn = Connection::open(db_path.as_ref())
            .with_context(|| format!("open notes database {}", db_path.as_ref().display()))?;
        let db = Self { conn };
        db.create_schema(schema_path)?;
        Ok(db)
    }

    pub fn connection(&self) -> &Connection {
        &self.conn
    }

    pub fn upsert_parsed_note(&mut self, path: &str, note: &ParsedNote) -> Result<i64> {
        let path = path.trim();
        if path.is_empty() {
            bail!("path is required");
        }

        let tx = self.conn.transaction()?;
        tx.execute(
            "INSERT INTO notes (path, title, content, updated_at)
             VALUES (?1, ?2, ?3, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
             ON CONFLICT(path) DO UPDATE SET
               title = excluded.title,
               content = excluded.content,
               updated_at = excluded.updated_at",
            params![path, note.title, note.content],
        )?;

        let note_id: i64 = tx.query_row("SELECT id FROM notes WHERE path = ?1", [path], |row| {
            row.get(0)
        })?;

        clear_note_children(&tx, note_id)?;
        insert_domains(&tx, note_id, &note.domains)?;
        insert_tags(&tx, note_id, &note.tags)?;
        insert_links(&tx, note_id, &note.links)?;
        upsert_recipe_projection_if_needed(&tx, note_id, note)?;
        enqueue_outbox_operation_tx(
            &tx,
            "note",
            path,
            "upsert",
            &note_sync_payload(path, note),
            None,
        )?;

        tx.commit()?;
        Ok(note_id)
    }

    pub fn all_note_paths(&self) -> Result<Vec<String>> {
        let mut stmt = self.conn.prepare("SELECT path FROM notes")?;
        let rows = stmt.query_map([], |row| row.get(0))?;
        let mut out = Vec::new();
        for row in rows {
            out.push(row?);
        }
        Ok(out)
    }

    pub fn delete_note_by_path(&self, path: &str) -> Result<()> {
        let path = path.trim();
        self.conn
            .execute("DELETE FROM notes WHERE path = ?1", [path])?;
        enqueue_outbox_operation_conn(
            &self.conn,
            "note",
            path,
            "delete",
            &serde_json::json!({ "path": path }).to_string(),
            None,
        )?;
        Ok(())
    }

    pub fn enqueue_outbox_operation(
        &self,
        entity_type: &str,
        entity_key: &str,
        op_type: &str,
        payload: &str,
        base_version: Option<i64>,
    ) -> Result<()> {
        enqueue_outbox_operation_conn(
            &self.conn,
            entity_type,
            entity_key,
            op_type,
            payload,
            base_version,
        )
    }

    pub fn list_pending_outbox(&self, limit: i64) -> Result<Vec<SyncOutboxRecord>> {
        let limit = if limit <= 0 { 100 } else { limit };
        let mut stmt = self.conn.prepare(
            "SELECT id, op_id, idempotency_key, entity_type, entity_key, op_type,
                    COALESCE(payload,''), payload_hash, base_version, status, attempt_count,
                    COALESCE(last_error,''), COALESCE(created_at,''), COALESCE(updated_at,''), acked_at
             FROM sync_outbox
             WHERE status = 'pending'
             ORDER BY id ASC
             LIMIT ?1",
        )?;
        let rows = stmt.query_map([limit], sync_outbox_record_from_row)?;
        collect_records(rows)
    }

    pub fn mark_outbox_acked(&self, op_id: &str) -> Result<()> {
        self.conn.execute(
            "UPDATE sync_outbox
             SET status = 'acked', acked_at = strftime('%Y-%m-%dT%H:%M:%SZ','now'),
                 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now'), last_error = NULL
             WHERE op_id = ?1",
            [op_id.trim()],
        )?;
        Ok(())
    }

    pub fn mark_outbox_attempt_failed(&self, op_id: &str, reason: &str) -> Result<()> {
        self.conn.execute(
            "UPDATE sync_outbox
             SET status = 'pending', attempt_count = attempt_count + 1, last_error = ?2,
                 updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
             WHERE op_id = ?1",
            params![op_id.trim(), reason.trim()],
        )?;
        Ok(())
    }

    pub fn set_sync_state(&self, key: &str, value: &str) -> Result<()> {
        self.conn.execute(
            "INSERT INTO sync_state (key, value, updated_at)
             VALUES (?1, ?2, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
             ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at",
            params![key.trim(), value.trim()],
        )?;
        Ok(())
    }

    pub fn get_sync_state(&self, key: &str) -> Result<Option<String>> {
        let result = self.conn.query_row(
            "SELECT value FROM sync_state WHERE key = ?1",
            [key.trim()],
            |row| row.get(0),
        );
        match result {
            Ok(value) => Ok(Some(value)),
            Err(rusqlite::Error::QueryReturnedNoRows) => Ok(None),
            Err(err) => Err(err.into()),
        }
    }

    pub fn sync_diagnostics(&self) -> Result<SyncDiagnostics> {
        let mut diag = SyncDiagnostics {
            local_cursor: self
                .get_sync_state(SYNC_STATE_LAST_SERVER_CURSOR_KEY)?
                .filter(|v| !v.trim().is_empty())
                .unwrap_or_else(|| "0".to_string()),
            ..SyncDiagnostics::default()
        };
        diag.pending_outbox_count = count_where(&self.conn, "sync_outbox", "status = 'pending'")?;
        diag.pending_outbox_retried_count = count_where(
            &self.conn,
            "sync_outbox",
            "status = 'pending' AND attempt_count > 0",
        )?;
        diag.pending_outbox_max_attempt_count = self.conn.query_row(
            "SELECT COALESCE(MAX(attempt_count), 0) FROM sync_outbox WHERE status = 'pending'",
            [],
            |row| row.get(0),
        )?;
        diag.pending_outbox_oldest_created_at = self.conn.query_row(
            "SELECT COALESCE(MIN(created_at), '') FROM sync_outbox WHERE status = 'pending'",
            [],
            |row| row.get(0),
        )?;
        diag.pending_outbox_latest_failure = self.conn.query_row(
            "SELECT COALESCE((SELECT last_error FROM sync_outbox WHERE status = 'pending' AND attempt_count > 0 ORDER BY updated_at DESC, id DESC LIMIT 1), '')",
            [],
            |row| row.get(0),
        )?;
        diag.acked_outbox_count = count_where(&self.conn, "sync_outbox", "status = 'acked'")?;
        diag.total_conflict_count = count_where(&self.conn, "sync_conflicts", "1 = 1")?;
        diag.unresolved_conflict_count =
            count_where(&self.conn, "sync_conflicts", "resolved_at IS NULL")?;
        diag.oldest_unresolved_conflict_at = self.conn.query_row(
            "SELECT COALESCE(MIN(created_at), '') FROM sync_conflicts WHERE resolved_at IS NULL",
            [],
            |row| row.get(0),
        )?;
        diag.sync_entity_version_count = count_where(&self.conn, "sync_entity_versions", "1 = 1")?;
        diag.synced_todo_count = count_where(&self.conn, "sync_todos", "1 = 1")?;
        Ok(diag)
    }

    pub fn count_notes(&self) -> Result<i64> {
        Ok(self
            .conn
            .query_row("SELECT COUNT(*) FROM notes", [], |row| row.get(0))?)
    }

    pub fn note_id_by_path(&self, path: &str) -> Result<Option<i64>> {
        let result = self.conn.query_row(
            "SELECT id FROM notes WHERE path = ?1",
            [path.trim()],
            |row| row.get(0),
        );
        match result {
            Ok(id) => Ok(Some(id)),
            Err(rusqlite::Error::QueryReturnedNoRows) => Ok(None),
            Err(err) => Err(err.into()),
        }
    }

    pub fn replace_registry(
        &mut self,
        entries: &[RegisteredNoteId],
        conflicts: &[RegistryConflict],
    ) -> Result<()> {
        let tx = self.conn.transaction()?;
        tx.execute("DELETE FROM note_ids", [])?;
        tx.execute("DELETE FROM note_id_conflicts", [])?;

        for entry in entries {
            let Some(note_id) = entry.note_id else {
                continue;
            };
            let uid = entry.uid.trim();
            if uid.is_empty() {
                continue;
            }
            tx.execute(
                "INSERT INTO note_ids (note_id, note_uid, path, is_hub, updated_at)
                 VALUES (?1, ?2, ?3, ?4, CURRENT_TIMESTAMP)",
                params![note_id, uid, entry.path, bool_to_int(entry.is_hub)],
            )
            .with_context(|| format!("insert note_ids ({})", entry.path))?;
        }

        for conflict in conflicts {
            let reason = normalize_reason(&conflict.reason);
            tx.execute(
                "INSERT INTO note_id_conflicts (note_id, note_uid, path, is_hub, reason, detected_at)
                 VALUES (?1, ?2, ?3, ?4, ?5, CURRENT_TIMESTAMP)",
                params![
                    conflict.note_id,
                    conflict.uid.as_deref().filter(|uid| !uid.trim().is_empty()),
                    conflict.path,
                    bool_to_int(conflict.is_hub),
                    reason,
                ],
            )
            .with_context(|| format!("insert note_id_conflicts ({})", conflict.path))?;
        }

        tx.commit()?;
        Ok(())
    }

    pub fn count_registry_entries(&self) -> Result<i64> {
        Ok(self
            .conn
            .query_row("SELECT COUNT(*) FROM note_ids", [], |row| row.get(0))?)
    }

    pub fn count_registry_conflicts(&self) -> Result<i64> {
        Ok(self
            .conn
            .query_row("SELECT COUNT(*) FROM note_id_conflicts", [], |row| {
                row.get(0)
            })?)
    }

    pub fn list_registry_entries(&self) -> Result<Vec<RegistryEntryRecord>> {
        let mut stmt = self.conn.prepare(
            "SELECT note_id, note_uid, path, is_hub, COALESCE(updated_at,'')
             FROM note_ids
             ORDER BY note_uid COLLATE NOCASE, path COLLATE NOCASE",
        )?;
        let rows = stmt.query_map([], |row| {
            Ok(RegistryEntryRecord {
                note_id: row.get(0)?,
                uid: row.get(1)?,
                path: row.get(2)?,
                is_hub: int_to_bool(row.get(3)?),
                updated_at: row.get(4)?,
            })
        })?;
        collect_records(rows)
    }

    pub fn list_registry_conflicts(&self) -> Result<Vec<RegistryConflictRecord>> {
        let mut stmt = self.conn.prepare(
            "SELECT note_id, note_uid, path, is_hub, reason, COALESCE(detected_at,'')
             FROM note_id_conflicts
             ORDER BY path COLLATE NOCASE, reason, note_uid",
        )?;
        let rows = stmt.query_map([], |row| {
            Ok(RegistryConflictRecord {
                note_id: row.get(0)?,
                uid: row.get(1)?,
                path: row.get(2)?,
                is_hub: int_to_bool(row.get(3)?),
                reason: row.get(4)?,
                detected_at: row.get(5)?,
            })
        })?;
        collect_records(rows)
    }

    pub fn list_notes(&self, limit: i64, offset: i64) -> Result<Vec<NoteRecord>> {
        let limit = if limit <= 0 { 50 } else { limit };
        let offset = offset.max(0);
        let mut stmt = self.conn.prepare(
            "SELECT id, path, COALESCE(title,''), COALESCE(content,''), COALESCE(updated_at,'')
             FROM notes
             ORDER BY updated_at DESC, id DESC
             LIMIT ?1 OFFSET ?2",
        )?;
        let rows = stmt.query_map(params![limit, offset], |row| note_record_from_row(row))?;
        self.collect_note_records(rows)
    }

    pub fn get_note_by_id(&self, id: i64) -> Result<Option<NoteRecord>> {
        let result = self.conn.query_row(
            "SELECT id, path, COALESCE(title,''), COALESCE(content,''), COALESCE(updated_at,'')
             FROM notes WHERE id = ?1",
            [id],
            |row| note_record_from_row(row),
        );
        match result {
            Ok(record) => Ok(Some(self.hydrate_note_record(record)?)),
            Err(rusqlite::Error::QueryReturnedNoRows) => Ok(None),
            Err(err) => Err(err.into()),
        }
    }

    pub fn get_note_by_uid(&self, uid: &str) -> Result<Option<NoteRecord>> {
        let Some(note_id) = self.note_id_by_uid(uid)? else {
            return Ok(None);
        };
        let mut record = self.get_note_by_id(note_id)?;
        if let Some(record) = &mut record {
            record.uid = uid.trim().to_string();
        }
        Ok(record)
    }

    pub fn search_notes_by_title(&self, query: &str) -> Result<Vec<NoteRecord>> {
        let mut stmt = self.conn.prepare(
            "SELECT DISTINCT id, path, COALESCE(title,''), COALESCE(content,''), COALESCE(updated_at,'')
             FROM notes
             WHERE LOWER(title) LIKE '%' || LOWER(?1) || '%'",
        )?;
        let rows = stmt.query_map([query], |row| note_record_from_row(row))?;
        self.collect_note_records(rows)
    }

    pub fn list_notes_by_tags(&self, tags: &[String]) -> Result<Vec<NoteRecord>> {
        let tags: Vec<String> = tags
            .iter()
            .map(|tag| tag.trim().to_string())
            .filter(|tag| !tag.is_empty())
            .collect();
        if tags.is_empty() {
            return Ok(Vec::new());
        }
        let placeholders = std::iter::repeat_n("?", tags.len())
            .collect::<Vec<_>>()
            .join(",");
        let sql = format!(
            "SELECT DISTINCT n.id, n.path, COALESCE(n.title,''), COALESCE(n.content,''), COALESCE(n.updated_at,'')
             FROM notes n
             JOIN tags t ON n.id = t.note_id
             WHERE t.tag IN ({placeholders})"
        );
        let mut stmt = self.conn.prepare(&sql)?;
        let rows = stmt.query_map(rusqlite::params_from_iter(tags), |row| {
            note_record_from_row(row)
        })?;
        self.collect_note_records(rows)
    }

    pub fn list_notes_by_domain(&self, domain: &str) -> Result<Vec<NoteRecord>> {
        let domain = domain.trim();
        if domain.is_empty() {
            return Ok(Vec::new());
        }
        let mut stmt = self.conn.prepare(
            "SELECT DISTINCT n.id, n.path, COALESCE(n.title,''), COALESCE(n.content,''), COALESCE(n.updated_at,'')
             FROM notes n
             JOIN note_domains d ON n.id = d.note_id
             WHERE d.domain = ?1
             ORDER BY n.updated_at DESC, n.id DESC",
        )?;
        let rows = stmt.query_map([domain], |row| note_record_from_row(row))?;
        self.collect_note_records(rows)
    }

    pub fn list_domains(&self) -> Result<Vec<String>> {
        let mut stmt = self.conn.prepare(
            "SELECT DISTINCT domain
             FROM note_domains
             WHERE TRIM(domain) != ''
             ORDER BY domain",
        )?;
        collect_strings(stmt.query_map([], |row| row.get(0))?)
    }

    pub fn list_recipes(&self, scope_domains: &[String]) -> Result<Vec<RecipeRecord>> {
        let scope_domains = normalize_scope_domains(scope_domains);
        let mut sql = String::from(
            "SELECT r.id, r.note_id, r.name, n.path, COALESCE(r.serving_size,''), COALESCE(r.prep_time,''),
                    COALESCE(r.cooking_time,''), COALESCE(r.meal,''), COALESCE(r.instructions,''),
                    COALESCE(r.payload_json,'{}'), COALESCE(r.updated_at,'')
             FROM recipes r
             JOIN notes n ON n.id = r.note_id",
        );
        for idx in 0..scope_domains.len() {
            sql.push_str(&format!(
                " JOIN note_domains scope_{idx} ON scope_{idx}.note_id = n.id AND scope_{idx}.domain = ?"
            ));
        }
        sql.push_str(" ORDER BY r.name COLLATE NOCASE");
        let mut stmt = self.conn.prepare(&sql)?;
        let rows = stmt.query_map(rusqlite::params_from_iter(scope_domains), |row| {
            Ok(RecipeRecord {
                id: row.get(0)?,
                note_id: row.get(1)?,
                name: row.get(2)?,
                path: row.get(3)?,
                serving_size: row.get(4)?,
                prep_time: row.get(5)?,
                cooking_time: row.get(6)?,
                meal: row.get(7)?,
                instructions: row.get(8)?,
                payload_json: row.get(9)?,
                updated_at: row.get(10)?,
            })
        })?;
        collect_records(rows)
    }

    pub fn list_ingredients(&self) -> Result<Vec<IngredientRecord>> {
        let mut stmt = self.conn.prepare(
            "SELECT i.id, i.name, COALESCE(i.ingredient_type,''), COALESCE(i.notes,''),
                    COUNT(DISTINCT rim.recipe_id) AS recipe_count,
                    COUNT(rim.id) AS mention_count,
                    COALESCE(i.created_at,''), COALESCE(i.updated_at,'')
             FROM ingredients i
             LEFT JOIN recipe_ingredient_mentions rim ON rim.canonical_ingredient_id = i.id
             GROUP BY i.id
             ORDER BY i.name COLLATE NOCASE",
        )?;
        let rows = stmt.query_map([], |row| {
            Ok(IngredientRecord {
                id: row.get(0)?,
                name: row.get(1)?,
                ingredient_type: row.get(2)?,
                notes: row.get(3)?,
                recipe_count: row.get(4)?,
                mention_count: row.get(5)?,
                created_at: row.get(6)?,
                updated_at: row.get(7)?,
            })
        })?;
        collect_records(rows)
    }

    pub fn list_ingredient_mentions(
        &self,
        unresolved_only: bool,
    ) -> Result<Vec<IngredientMentionRecord>> {
        let mut sql = String::from(
            "SELECT rim.id, rim.note_id, rim.recipe_id, r.name, n.path, rim.raw_text, rim.raw_name,
                    COALESCE(rim.quantity_text,''), rim.quantity_number, COALESCE(rim.unit_raw,''),
                    rim.canonical_ingredient_id, COALESCE(i.name,''), rim.line_number
             FROM recipe_ingredient_mentions rim
             JOIN recipes r ON r.id = rim.recipe_id
             JOIN notes n ON n.id = rim.note_id
             LEFT JOIN ingredients i ON i.id = rim.canonical_ingredient_id",
        );
        if unresolved_only {
            sql.push_str(" WHERE rim.canonical_ingredient_id IS NULL");
        }
        sql.push_str(" ORDER BY r.name COLLATE NOCASE, rim.line_number ASC, rim.id ASC");
        let mut stmt = self.conn.prepare(&sql)?;
        let rows = stmt.query_map([], |row| {
            Ok(IngredientMentionRecord {
                id: row.get(0)?,
                note_id: row.get(1)?,
                recipe_id: row.get(2)?,
                recipe_name: row.get(3)?,
                path: row.get(4)?,
                raw_text: row.get(5)?,
                raw_name: row.get(6)?,
                quantity_text: row.get(7)?,
                quantity_number: row.get(8)?,
                unit_raw: row.get(9)?,
                canonical_ingredient_id: row.get(10)?,
                canonical_name: row.get(11)?,
                line_number: row.get(12)?,
            })
        })?;
        collect_records(rows)
    }

    pub fn query_graph(
        &self,
        search: &str,
        domain: &str,
        depth: i64,
        limit: i64,
    ) -> Result<GraphResult> {
        let depth = depth.max(0);
        let limit = if limit <= 0 { 250 } else { limit as usize };
        let node_rows = self.list_graph_nodes()?;
        let edge_rows = self.list_graph_edges()?;
        let nodes_by_id: std::collections::BTreeMap<i64, GraphNodeRow> = node_rows
            .iter()
            .map(|node| (node.id, node.clone()))
            .collect();

        let search_norm = search.trim().to_ascii_lowercase();
        let domain_norm = domain.trim().to_ascii_lowercase();
        let mut seed = std::collections::BTreeSet::new();
        if search_norm.is_empty() && domain_norm.is_empty() {
            for node in &node_rows {
                seed.insert(node.id);
                if seed.len() >= limit {
                    break;
                }
            }
        } else {
            for node in &node_rows {
                if graph_node_matches(node, &search_norm, &domain_norm) {
                    seed.insert(node.id);
                    if seed.len() >= limit {
                        break;
                    }
                }
            }
        }

        let included = expand_graph_nodes(&seed, &edge_rows, depth, limit);
        if included.is_empty() {
            return Ok(GraphResult {
                nodes: Vec::new(),
                edges: Vec::new(),
                meta: GraphMeta {
                    seed_count: seed.len(),
                    depth,
                    search: search.to_string(),
                    domain: domain.to_string(),
                },
            });
        }

        let mut nodes = Vec::new();
        for id in &included {
            let Some(row) = nodes_by_id.get(id) else {
                continue;
            };
            let matched = seed.contains(id);
            let uid = row.uid.trim().to_string();
            nodes.push(GraphNode {
                id: graph_node_id(*id),
                note_id: *id,
                uid: uid.clone(),
                label: if uid.is_empty() {
                    "unknown".to_string()
                } else {
                    uid.clone()
                },
                title: row.title.clone(),
                path: row.path.clone(),
                kind: "note".to_string(),
                tags: row.tags.clone(),
                domains: row.domains.clone(),
                matched,
                unknown: uid.is_empty(),
            });
        }
        nodes.sort_by(|a, b| {
            a.label
                .to_ascii_lowercase()
                .cmp(&b.label.to_ascii_lowercase())
        });

        let mut edges = Vec::new();
        let mut seen_edges = std::collections::BTreeSet::new();
        for edge in edge_rows {
            if !included.contains(&edge.source_id) || !included.contains(&edge.target_id) {
                continue;
            }
            let id = format!(
                "{}->{}:{}",
                graph_node_id(edge.source_id),
                graph_node_id(edge.target_id),
                edge.label
            );
            if !seen_edges.insert(id.clone()) {
                continue;
            }
            edges.push(GraphEdge {
                id,
                source: graph_node_id(edge.source_id),
                target: graph_node_id(edge.target_id),
                kind: "mentions".to_string(),
                label: edge.label,
            });
        }

        Ok(GraphResult {
            nodes,
            edges,
            meta: GraphMeta {
                seed_count: seed.len(),
                depth,
                search: search.to_string(),
                domain: domain.to_string(),
            },
        })
    }

    fn list_graph_nodes(&self) -> Result<Vec<GraphNodeRow>> {
        let mut stmt = self.conn.prepare(
            "SELECT n.id, COALESCE(ni.note_uid,''), COALESCE(n.title,''), n.path
             FROM notes n
             LEFT JOIN note_ids ni ON ni.note_id = n.id
             ORDER BY COALESCE(ni.note_uid,''), COALESCE(n.title,''), n.id",
        )?;
        let rows = stmt.query_map([], |row| {
            Ok(GraphNodeRow {
                id: row.get(0)?,
                uid: row.get(1)?,
                title: row.get(2)?,
                path: row.get(3)?,
                tags: Vec::new(),
                domains: Vec::new(),
            })
        })?;
        let mut out = Vec::new();
        for row in rows {
            let mut row = row?;
            row.tags = self.tags_for_note(row.id)?;
            row.domains = self.domains_for_note(row.id)?;
            out.push(row);
        }
        Ok(out)
    }

    fn list_graph_edges(&self) -> Result<Vec<GraphEdgeRow>> {
        let mut stmt = self.conn.prepare(
            "WITH resolved_links AS (
                SELECT
                    l.id,
                    l.note_id,
                    COALESCE(l.label,'') AS label,
                    COALESCE(
                        (SELECT n.id FROM notes n WHERE n.path = l.resolved_path LIMIT 1),
                        (SELECT n.id FROM notes n WHERE n.path = l.resolved_path || '.md' LIMIT 1),
                        (SELECT n.id FROM notes n WHERE n.path = l.resolved_path || '/hub.md' LIMIT 1),
                        (SELECT ni.note_id FROM note_ids ni WHERE lower(ni.note_uid) = lower(trim(l.target)) ORDER BY ni.note_id LIMIT 1),
                        (SELECT ni.note_id FROM note_ids ni WHERE lower(ni.note_uid) = lower(trim(l.resolved_path)) ORDER BY ni.note_id LIMIT 1)
                    ) AS target_note_id
                FROM links l
                WHERE l.type = 'internal'
             )
             SELECT rl.note_id, dst.id, rl.label
             FROM resolved_links rl
             JOIN notes dst ON dst.id = rl.target_note_id
             ORDER BY rl.note_id, dst.id, rl.id",
        )?;
        let rows = stmt.query_map([], |row| {
            Ok(GraphEdgeRow {
                source_id: row.get(0)?,
                target_id: row.get(1)?,
                label: row.get(2)?,
            })
        })?;
        collect_records(rows)
    }

    fn note_id_by_uid(&self, uid: &str) -> Result<Option<i64>> {
        let uid = uid.trim();
        if uid.is_empty() {
            return Ok(None);
        }
        let result = self.conn.query_row(
            "SELECT note_id FROM note_ids WHERE note_uid = ?1",
            [uid],
            |row| row.get(0),
        );
        match result {
            Ok(id) => Ok(Some(id)),
            Err(rusqlite::Error::QueryReturnedNoRows) => Ok(None),
            Err(err) => Err(err.into()),
        }
    }

    fn collect_note_records<'stmt>(
        &self,
        rows: impl Iterator<Item = rusqlite::Result<NoteRecord>>,
    ) -> Result<Vec<NoteRecord>> {
        let mut out = Vec::new();
        for row in rows {
            out.push(self.hydrate_note_record(row?)?);
        }
        Ok(out)
    }

    fn hydrate_note_record(&self, mut record: NoteRecord) -> Result<NoteRecord> {
        record.tags = self.tags_for_note(record.id)?;
        record.domains = self.domains_for_note(record.id)?;
        record.links = self.links_for_note(record.id)?;
        Ok(record)
    }

    fn tags_for_note(&self, note_id: i64) -> Result<Vec<String>> {
        let mut stmt = self
            .conn
            .prepare("SELECT tag FROM tags WHERE note_id = ?1 ORDER BY tag")?;
        collect_strings(stmt.query_map([note_id], |row| row.get(0))?)
    }

    fn domains_for_note(&self, note_id: i64) -> Result<Vec<String>> {
        let mut stmt = self
            .conn
            .prepare("SELECT domain FROM note_domains WHERE note_id = ?1 ORDER BY domain")?;
        collect_strings(stmt.query_map([note_id], |row| row.get(0))?)
    }

    fn links_for_note(&self, note_id: i64) -> Result<Vec<mw_notes::Link>> {
        let mut stmt = self.conn.prepare(
            "SELECT COALESCE(label,''), COALESCE(target,''), COALESCE(type,''), COALESCE(resolved_path,'')
             FROM links
             WHERE note_id = ?1
             ORDER BY id ASC",
        )?;
        let rows = stmt.query_map([note_id], |row| {
            let typ: String = row.get(2)?;
            Ok(mw_notes::Link {
                label: row.get(0)?,
                target: row.get(1)?,
                link_type: if typ == "external" {
                    LinkType::External
                } else {
                    LinkType::Internal
                },
                resolved_path: row.get(3)?,
            })
        })?;
        let mut out = Vec::new();
        for row in rows {
            out.push(row?);
        }
        Ok(out)
    }

    fn create_schema(&self, schema_path: Option<&str>) -> Result<()> {
        let schema = read_schema(schema_path, EMBEDDED_SCHEMA_PATH, EMBEDDED_NOTE_SCHEMA)?;
        if schema.trim().is_empty() {
            bail!(
                "schema is empty: {}",
                schema_path.unwrap_or(EMBEDDED_SCHEMA_PATH)
            );
        }

        self.conn.execute_batch("PRAGMA foreign_keys = ON;")?;
        self.migrate_registry_tables_if_needed()?;
        self.conn.execute_batch(&schema).with_context(|| {
            format!(
                "apply schema {}",
                schema_path.unwrap_or(EMBEDDED_SCHEMA_PATH)
            )
        })?;
        self.migrate_sync_tables_if_needed()?;
        Ok(())
    }

    fn migrate_registry_tables_if_needed(&self) -> Result<()> {
        let note_ids_legacy = table_has_column(&self.conn, "note_ids", "is_index")?;
        let conflicts_legacy = table_has_column(&self.conn, "note_id_conflicts", "is_index")?;
        if !note_ids_legacy && !conflicts_legacy {
            return Ok(());
        }

        self.conn.execute_batch(
            "DROP TABLE IF EXISTS note_id_conflicts;\nDROP TABLE IF EXISTS note_ids;",
        )?;
        Ok(())
    }

    fn migrate_sync_tables_if_needed(&self) -> Result<()> {
        if !table_has_column(&self.conn, "sync_outbox", "base_version")? {
            self.conn.execute_batch(
                "ALTER TABLE sync_outbox ADD COLUMN base_version INTEGER NOT NULL DEFAULT 0;",
            )?;
        }
        ensure_sync_todos_columns(&self.conn)?;
        Ok(())
    }
}

fn normalize_reason(reason: &str) -> String {
    match reason.trim().to_ascii_uppercase().as_str() {
        "DUPLICATE_ID" | "MISSING_HUB_ID" | "NOTE_NOT_IN_DB" => reason.trim().to_ascii_uppercase(),
        _ => "DUPLICATE_ID".to_string(),
    }
}

fn bool_to_int(value: bool) -> i64 {
    if value { 1 } else { 0 }
}

fn int_to_bool(value: i64) -> bool {
    value != 0
}

fn clear_note_children(tx: &rusqlite::Transaction<'_>, note_id: i64) -> Result<()> {
    tx.execute(
        "DELETE FROM recipe_ingredient_mentions WHERE note_id = ?1",
        [note_id],
    )?;
    tx.execute("DELETE FROM recipes WHERE note_id = ?1", [note_id])?;
    tx.execute("DELETE FROM note_domains WHERE note_id = ?1", [note_id])?;
    tx.execute("DELETE FROM tags WHERE note_id = ?1", [note_id])?;
    tx.execute("DELETE FROM links WHERE note_id = ?1", [note_id])?;
    tx.execute("DELETE FROM todos WHERE note_id = ?1", [note_id])?;
    tx.execute("DELETE FROM task_groups WHERE note_id = ?1", [note_id])?;
    Ok(())
}

fn upsert_recipe_projection_if_needed(
    tx: &rusqlite::Transaction<'_>,
    note_id: i64,
    note: &ParsedNote,
) -> Result<()> {
    if !note
        .domains
        .iter()
        .any(|domain| domain.trim().eq_ignore_ascii_case("recipe"))
    {
        return Ok(());
    }

    let projection = mw_notes::extract_recipe_projection(&note.content, &note.title, &note.meta);
    let name = if projection.name.trim().is_empty() {
        "Untitled recipe"
    } else {
        projection.name.trim()
    };
    let payload = serde_json::to_string(&projection).unwrap_or_else(|_| "{}".to_string());
    tx.execute(
        "INSERT INTO recipes (note_id, name, serving_size, prep_time, cooking_time, meal, instructions, payload_json, updated_at)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, strftime('%Y-%m-%dT%H:%M:%SZ','now'))",
        params![
            note_id,
            name,
            projection.serving_size,
            projection.prep_time,
            projection.cooking_time,
            projection.meal.join(","),
            projection.instructions.join("\n"),
            payload,
        ],
    )?;
    let recipe_id = tx.last_insert_rowid();

    for ingredient in projection.ingredients {
        let raw_name = ingredient.raw_name.trim();
        if raw_name.is_empty() {
            continue;
        }
        let ingredient_id = upsert_ingredient(tx, raw_name)?;
        tx.execute(
            "INSERT INTO recipe_ingredient_mentions
               (note_id, recipe_id, raw_text, raw_name, quantity_text, quantity_number, unit_raw, canonical_ingredient_id, line_number)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)",
            params![
                note_id,
                recipe_id,
                ingredient.raw_text,
                raw_name,
                ingredient.quantity_text,
                ingredient.quantity_number,
                ingredient.unit_raw,
                ingredient_id,
                ingredient.line_number as i64,
            ],
        )?;
    }

    Ok(())
}

fn upsert_ingredient(tx: &rusqlite::Transaction<'_>, name: &str) -> Result<i64> {
    tx.execute(
        "INSERT INTO ingredients (name, updated_at)
         VALUES (?1, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
         ON CONFLICT(name) DO UPDATE SET updated_at=excluded.updated_at",
        [name.trim()],
    )?;
    Ok(tx.query_row(
        "SELECT id FROM ingredients WHERE name = ?1",
        [name.trim()],
        |row| row.get(0),
    )?)
}

fn insert_domains(tx: &rusqlite::Transaction<'_>, note_id: i64, domains: &[String]) -> Result<()> {
    for domain in domains {
        let domain = domain.trim();
        if domain.is_empty() {
            continue;
        }
        tx.execute(
            "INSERT OR IGNORE INTO note_domains (note_id, domain) VALUES (?1, ?2)",
            params![note_id, domain],
        )?;
    }
    Ok(())
}

fn insert_tags(tx: &rusqlite::Transaction<'_>, note_id: i64, tags: &[String]) -> Result<()> {
    for tag in tags {
        tx.execute(
            "INSERT INTO tags (note_id, tag) VALUES (?1, ?2)",
            params![note_id, tag],
        )?;
    }
    Ok(())
}

fn insert_links(
    tx: &rusqlite::Transaction<'_>,
    note_id: i64,
    links: &[mw_notes::Link],
) -> Result<()> {
    for link in links {
        let link_type = match link.link_type {
            LinkType::Internal => "internal",
            LinkType::External => "external",
        };
        tx.execute(
            "INSERT INTO links (note_id, label, target, type, resolved_path)
             VALUES (?1, ?2, ?3, ?4, ?5)",
            params![
                note_id,
                link.label,
                link.target,
                link_type,
                link.resolved_path
            ],
        )?;
    }
    Ok(())
}

fn note_record_from_row(row: &rusqlite::Row<'_>) -> rusqlite::Result<NoteRecord> {
    Ok(NoteRecord {
        id: row.get(0)?,
        path: row.get(1)?,
        title: row.get(2)?,
        content: row.get(3)?,
        updated_at: row.get(4)?,
        uid: String::new(),
        tags: Vec::new(),
        domains: Vec::new(),
        links: Vec::new(),
    })
}

fn collect_strings(rows: impl Iterator<Item = rusqlite::Result<String>>) -> Result<Vec<String>> {
    let mut out = Vec::new();
    for row in rows {
        out.push(row?);
    }
    Ok(out)
}

fn collect_records<T>(rows: impl Iterator<Item = rusqlite::Result<T>>) -> Result<Vec<T>> {
    let mut out = Vec::new();
    for row in rows {
        out.push(row?);
    }
    Ok(out)
}

fn sync_outbox_record_from_row(row: &rusqlite::Row<'_>) -> rusqlite::Result<SyncOutboxRecord> {
    Ok(SyncOutboxRecord {
        id: row.get(0)?,
        op_id: row.get(1)?,
        idempotency_key: row.get(2)?,
        entity_type: row.get(3)?,
        entity_key: row.get(4)?,
        op_type: row.get(5)?,
        payload: row.get(6)?,
        payload_hash: row.get(7)?,
        base_version: row.get(8)?,
        status: row.get(9)?,
        attempt_count: row.get(10)?,
        last_error: row.get(11)?,
        created_at: row.get(12)?,
        updated_at: row.get(13)?,
        acked_at: row.get(14)?,
    })
}

fn enqueue_outbox_operation_conn(
    conn: &Connection,
    entity_type: &str,
    entity_key: &str,
    op_type: &str,
    payload: &str,
    base_version: Option<i64>,
) -> Result<()> {
    enqueue_outbox_operation(
        conn,
        entity_type,
        entity_key,
        op_type,
        payload,
        base_version,
    )
}

fn enqueue_outbox_operation_tx(
    tx: &rusqlite::Transaction<'_>,
    entity_type: &str,
    entity_key: &str,
    op_type: &str,
    payload: &str,
    base_version: Option<i64>,
) -> Result<()> {
    enqueue_outbox_operation(tx, entity_type, entity_key, op_type, payload, base_version)
}

trait SqlExecutor {
    fn execute_sql<P: rusqlite::Params>(&self, sql: &str, params: P) -> rusqlite::Result<usize>;
}

impl SqlExecutor for Connection {
    fn execute_sql<P: rusqlite::Params>(&self, sql: &str, params: P) -> rusqlite::Result<usize> {
        self.execute(sql, params)
    }
}

impl SqlExecutor for rusqlite::Transaction<'_> {
    fn execute_sql<P: rusqlite::Params>(&self, sql: &str, params: P) -> rusqlite::Result<usize> {
        self.execute(sql, params)
    }
}

fn enqueue_outbox_operation(
    conn: &impl SqlExecutor,
    entity_type: &str,
    entity_key: &str,
    op_type: &str,
    payload: &str,
    base_version: Option<i64>,
) -> Result<()> {
    let entity_type = entity_type.trim();
    let entity_key = entity_key.trim();
    let op_type = op_type.trim();
    if !matches!(entity_type, "note" | "todo") {
        bail!("unsupported sync entity type: {entity_type}");
    }
    if !matches!(op_type, "upsert" | "delete") {
        bail!("unsupported sync operation type: {op_type}");
    }
    if entity_key.is_empty() {
        bail!("sync entity key is required");
    }

    let payload_hash = stable_hex_hash(payload);
    let base_version = base_version.unwrap_or(0).max(0);
    let idempotency_key = stable_hex_hash(&format!(
        "{entity_type}|{entity_key}|{op_type}|{base_version}|{payload_hash}"
    ));
    let op_id = new_sync_op_id();
    conn.execute_sql(
        "INSERT OR IGNORE INTO sync_outbox
         (op_id, idempotency_key, entity_type, entity_key, op_type, payload, payload_hash, base_version, status, attempt_count, created_at, updated_at)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, 'pending', 0,
                 strftime('%Y-%m-%dT%H:%M:%SZ','now'), strftime('%Y-%m-%dT%H:%M:%SZ','now'))",
        params![
            op_id,
            idempotency_key,
            entity_type,
            entity_key,
            op_type,
            payload,
            payload_hash,
            base_version,
        ],
    )?;
    Ok(())
}

fn note_sync_payload(path: &str, note: &ParsedNote) -> String {
    serde_json::json!({
        "path": path,
        "title": note.title,
        "content": note.content,
        "tags": note.tags,
        "domains": note.domains,
    })
    .to_string()
}

fn stable_hex_hash(value: &str) -> String {
    let mut hash = 0xcbf29ce484222325_u64;
    for byte in value.as_bytes() {
        hash ^= u64::from(*byte);
        hash = hash.wrapping_mul(0x100000001b3);
    }
    format!("{hash:016x}")
}

fn new_sync_op_id() -> String {
    let counter = OP_COUNTER.fetch_add(1, Ordering::Relaxed);
    let millis = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis())
        .unwrap_or_default();
    format!("mw-rust-{millis}-{counter}")
}

fn count_where(conn: &Connection, table: &str, where_clause: &str) -> Result<i64> {
    if !is_safe_identifier(table) {
        bail!("unsafe table name: {table}");
    }
    let sql = format!("SELECT COUNT(*) FROM {table} WHERE {where_clause}");
    Ok(conn.query_row(&sql, [], |row| row.get(0))?)
}

fn is_safe_identifier(value: &str) -> bool {
    !value.is_empty()
        && value
            .chars()
            .all(|ch| ch.is_ascii_alphanumeric() || ch == '_')
}

fn normalize_scope_domains(domains: &[String]) -> Vec<String> {
    let mut seen = std::collections::BTreeSet::new();
    let mut out = Vec::new();
    for domain in domains {
        let domain = domain.trim();
        if domain.is_empty() || !seen.insert(domain.to_string()) {
            continue;
        }
        out.push(domain.to_string());
    }
    out
}

fn graph_node_matches(node: &GraphNodeRow, search: &str, domain: &str) -> bool {
    if !domain.is_empty()
        && !node
            .domains
            .iter()
            .any(|d| d.trim().eq_ignore_ascii_case(domain))
    {
        return false;
    }
    if search.is_empty() {
        return true;
    }
    let haystack = format!(
        "{} {} {} {} {}",
        node.uid,
        node.title,
        node.path,
        node.tags.join(" "),
        node.domains.join(" ")
    )
    .to_ascii_lowercase();
    haystack.contains(search)
}

fn expand_graph_nodes(
    seed: &std::collections::BTreeSet<i64>,
    edges: &[GraphEdgeRow],
    depth: i64,
    limit: usize,
) -> std::collections::BTreeSet<i64> {
    let mut included = seed.clone();
    let mut frontier: Vec<i64> = seed.iter().copied().collect();
    let mut neighbors: std::collections::BTreeMap<i64, Vec<i64>> =
        std::collections::BTreeMap::new();
    for edge in edges {
        neighbors
            .entry(edge.source_id)
            .or_default()
            .push(edge.target_id);
        neighbors
            .entry(edge.target_id)
            .or_default()
            .push(edge.source_id);
    }

    for _ in 0..depth {
        if included.len() >= limit {
            break;
        }
        let mut next = Vec::new();
        for id in &frontier {
            let mut ns = neighbors.get(id).cloned().unwrap_or_default();
            ns.sort();
            for neighbor in ns {
                if included.contains(&neighbor) {
                    continue;
                }
                included.insert(neighbor);
                next.push(neighbor);
                if included.len() >= limit {
                    break;
                }
            }
            if included.len() >= limit {
                break;
            }
        }
        frontier = next;
        if frontier.is_empty() {
            break;
        }
    }
    included
}

fn graph_node_id(id: i64) -> String {
    format!("note:{id}")
}

impl CommandDb {
    pub fn open(db_path: impl AsRef<Path>, schema_path: Option<&str>) -> Result<Self> {
        ensure_parent_dir(db_path.as_ref())?;
        let conn = Connection::open(db_path.as_ref())
            .with_context(|| format!("open commands database {}", db_path.as_ref().display()))?;
        let db = Self { conn };
        db.create_schema(schema_path)?;
        Ok(db)
    }

    pub fn connection(&self) -> &Connection {
        &self.conn
    }

    fn create_schema(&self, schema_path: Option<&str>) -> Result<()> {
        let schema = read_schema(
            schema_path,
            EMBEDDED_COMMAND_SCHEMA_PATH,
            EMBEDDED_COMMAND_SCHEMA,
        )?;
        if schema.trim().is_empty() {
            bail!(
                "schema is empty: {}",
                schema_path.unwrap_or(EMBEDDED_COMMAND_SCHEMA_PATH)
            );
        }
        self.conn.execute_batch(&schema).with_context(|| {
            format!(
                "apply schema {}",
                schema_path.unwrap_or(EMBEDDED_COMMAND_SCHEMA_PATH)
            )
        })?;
        Ok(())
    }
}

pub fn initialize_from_config(cfg: &Config) -> Result<DbInitReport> {
    let _notes = NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    let _commands = CommandDb::open(&cfg.commands_db_path, Some(&cfg.commands_schema_path))?;
    Ok(DbInitReport {
        notes_db_path: cfg.db_path.clone(),
        commands_db_path: cfg.commands_db_path.clone(),
    })
}

pub fn validate_from_config(cfg: &Config) -> Result<()> {
    let notes = NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    require_table(notes.connection(), "notes")?;
    require_table(notes.connection(), "links")?;
    require_table(notes.connection(), "saved_queries")?;
    require_table(notes.connection(), "sync_outbox")?;

    let commands = CommandDb::open(&cfg.commands_db_path, Some(&cfg.commands_schema_path))?;
    require_table(commands.connection(), "tools")?;
    require_table(commands.connection(), "commands")?;
    Ok(())
}

fn read_schema(schema_path: Option<&str>, embedded_path: &str, embedded: &str) -> Result<String> {
    let schema_path = schema_path.unwrap_or_default().trim();
    if schema_path.is_empty() || schema_path == embedded_path {
        return Ok(embedded.to_string());
    }
    fs::read_to_string(schema_path).with_context(|| format!("read schema {schema_path}"))
}

fn ensure_parent_dir(path: &Path) -> Result<()> {
    let Some(parent) = path.parent() else {
        return Ok(());
    };
    if parent.as_os_str().is_empty() {
        return Ok(());
    }
    fs::create_dir_all(parent).with_context(|| format!("create directory {}", parent.display()))
}

fn table_has_column(conn: &Connection, table_name: &str, column_name: &str) -> Result<bool> {
    let mut stmt = conn.prepare(&format!("PRAGMA table_info({table_name})"))?;
    let mut rows = stmt.query([])?;
    while let Some(row) = rows.next()? {
        let name: String = row.get(1)?;
        if name.eq_ignore_ascii_case(column_name) {
            return Ok(true);
        }
    }
    Ok(false)
}

fn ensure_sync_todos_columns(conn: &Connection) -> Result<()> {
    let required = [
        ("source_id", "TEXT NOT NULL DEFAULT ''"),
        ("source_path", "TEXT"),
        ("task_scope", "TEXT"),
        ("task_area", "TEXT"),
        ("todo_section", "TEXT NOT NULL DEFAULT 'Inbox'"),
        ("task_text", "TEXT NOT NULL DEFAULT ''"),
        ("is_done", "INTEGER NOT NULL DEFAULT 0"),
        ("meta", "TEXT"),
        ("task_order", "INTEGER NOT NULL DEFAULT 0"),
        ("line_number", "INTEGER"),
    ];

    for (name, definition) in required {
        if table_has_column(conn, "sync_todos", name)? {
            continue;
        }
        conn.execute_batch(&format!(
            "ALTER TABLE sync_todos ADD COLUMN {name} {definition};"
        ))?;
    }

    conn.execute_batch(
        "CREATE INDEX IF NOT EXISTS idx_sync_todos_updated ON sync_todos(updated_at DESC);",
    )?;
    Ok(())
}

fn require_table(conn: &Connection, table_name: &str) -> Result<()> {
    let exists: i64 = conn.query_row(
        "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?1",
        [table_name],
        |row| row.get(0),
    )?;
    if exists == 0 {
        return Err(anyhow!("missing SQLite table: {table_name}"));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn initializes_note_database_with_expected_tables() {
        let temp = tempfile::tempdir().unwrap();
        let db_path = temp.path().join("notes.db");
        let db = NoteDb::open(&db_path, Some(EMBEDDED_SCHEMA_PATH)).unwrap();

        require_table(db.connection(), "notes").unwrap();
        require_table(db.connection(), "note_ids").unwrap();
        require_table(db.connection(), "recipes").unwrap();

        let saved_queries: i64 = db
            .connection()
            .query_row("SELECT COUNT(*) FROM saved_queries", [], |row| row.get(0))
            .unwrap();
        assert!(saved_queries >= 3);
    }

    #[test]
    fn initializes_command_database_with_expected_tables() {
        let temp = tempfile::tempdir().unwrap();
        let db_path = temp.path().join("command.db");
        let db = CommandDb::open(&db_path, Some(EMBEDDED_COMMAND_SCHEMA_PATH)).unwrap();

        require_table(db.connection(), "tools").unwrap();
        require_table(db.connection(), "commands").unwrap();
        require_table(db.connection(), "examples").unwrap();
    }

    #[test]
    fn enqueues_deduplicated_sync_outbox_operations() {
        let temp = tempfile::tempdir().unwrap();
        let db_path = temp.path().join("notes.db");
        let mut db = NoteDb::open(&db_path, Some(EMBEDDED_SCHEMA_PATH)).unwrap();
        let parsed = mw_notes::parse_note(
            r#"---
domains: [writing]
tags: [sync]
---

# Sync note

Body
"#,
            "sync/note.md",
        );

        db.upsert_parsed_note("sync/note.md", &parsed).unwrap();
        db.upsert_parsed_note("sync/note.md", &parsed).unwrap();

        let pending = db.list_pending_outbox(10).unwrap();
        assert_eq!(pending.len(), 1);
        assert_eq!(pending[0].entity_type, "note");
        assert_eq!(pending[0].entity_key, "sync/note.md");
        assert_eq!(pending[0].op_type, "upsert");
        assert_eq!(pending[0].base_version, 0);
        assert!(pending[0].payload.contains("Sync note"));
    }

    #[test]
    fn tracks_sync_outbox_ack_fail_and_diagnostics() {
        let temp = tempfile::tempdir().unwrap();
        let db_path = temp.path().join("notes.db");
        let db = NoteDb::open(&db_path, Some(EMBEDDED_SCHEMA_PATH)).unwrap();

        db.enqueue_outbox_operation("todo", "todo-1", "upsert", r#"{"id":"todo-1"}"#, Some(7))
            .unwrap();
        let op = db.list_pending_outbox(10).unwrap().remove(0);
        assert_eq!(op.base_version, 7);

        db.mark_outbox_attempt_failed(&op.op_id, "transient")
            .unwrap();
        let retried = db.list_pending_outbox(10).unwrap().remove(0);
        assert_eq!(retried.attempt_count, 1);
        assert_eq!(retried.last_error, "transient");

        let diag = db.sync_diagnostics().unwrap();
        assert_eq!(diag.pending_outbox_count, 1);
        assert_eq!(diag.pending_outbox_retried_count, 1);
        assert_eq!(diag.pending_outbox_max_attempt_count, 1);
        assert_eq!(diag.pending_outbox_latest_failure, "transient");

        db.mark_outbox_acked(&retried.op_id).unwrap();
        let diag = db.sync_diagnostics().unwrap();
        assert_eq!(diag.pending_outbox_count, 0);
        assert_eq!(diag.acked_outbox_count, 1);
    }

    #[test]
    fn migrates_legacy_registry_tables() {
        let temp = tempfile::tempdir().unwrap();
        let db_path = temp.path().join("legacy.db");
        {
            let conn = Connection::open(&db_path).unwrap();
            conn.execute_batch(
                "CREATE TABLE note_ids (note_id INTEGER, is_index INTEGER);\n\
                 CREATE TABLE note_id_conflicts (note_id INTEGER, is_index INTEGER);",
            )
            .unwrap();
        }

        let db = NoteDb::open(&db_path, Some(EMBEDDED_SCHEMA_PATH)).unwrap();
        assert!(!table_has_column(db.connection(), "note_ids", "is_index").unwrap());
        assert!(table_has_column(db.connection(), "note_ids", "is_hub").unwrap());
    }

    #[test]
    fn upserts_parsed_note_children_and_replaces_old_children() {
        let temp = tempfile::tempdir().unwrap();
        let db_path = temp.path().join("notes.db");
        let mut db = NoteDb::open(&db_path, Some(EMBEDDED_SCHEMA_PATH)).unwrap();

        let first = mw_notes::parse_note(
            r#"---
domains: [glossary]
tags: [first]
---

See [[alpha]] and [External](https://example.com).
"#,
            "area/hub.md",
        );
        let note_id = db.upsert_parsed_note("area/hub.md", &first).unwrap();

        let domains: i64 = db
            .connection()
            .query_row(
                "SELECT COUNT(*) FROM note_domains WHERE note_id = ?1 AND domain = 'glossary'",
                [note_id],
                |row| row.get(0),
            )
            .unwrap();
        let links: i64 = db
            .connection()
            .query_row(
                "SELECT COUNT(*) FROM links WHERE note_id = ?1",
                [note_id],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(domains, 1);
        assert_eq!(links, 2);

        let second = mw_notes::parse_note(
            r#"---
domains: [recipe]
tags: [second]
---

No links.
"#,
            "area/hub.md",
        );
        let second_id = db.upsert_parsed_note("area/hub.md", &second).unwrap();
        assert_eq!(note_id, second_id);

        let old_domains: i64 = db
            .connection()
            .query_row(
                "SELECT COUNT(*) FROM note_domains WHERE note_id = ?1 AND domain = 'glossary'",
                [note_id],
                |row| row.get(0),
            )
            .unwrap();
        let new_domains: i64 = db
            .connection()
            .query_row(
                "SELECT COUNT(*) FROM note_domains WHERE note_id = ?1 AND domain = 'recipe'",
                [note_id],
                |row| row.get(0),
            )
            .unwrap();
        let links_after: i64 = db
            .connection()
            .query_row(
                "SELECT COUNT(*) FROM links WHERE note_id = ?1",
                [note_id],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(old_domains, 0);
        assert_eq!(new_domains, 1);
        assert_eq!(links_after, 0);
    }

    #[test]
    fn upserts_and_queries_recipe_projection() {
        let temp = tempfile::tempdir().unwrap();
        let db_path = temp.path().join("notes.db");
        let mut db = NoteDb::open(&db_path, Some(EMBEDDED_SCHEMA_PATH)).unwrap();

        let parsed = mw_notes::parse_note(
            r#"---
domains: [recipe, dinner]
servings: 4
meal: [dinner]
---

# Lentil Soup

## Ingredients
- 1 1/2 cups lentils
- salt

## Instructions
1. Rinse lentils
- Simmer
"#,
            "recipes/lentil-soup.md",
        );
        let note_id = db
            .upsert_parsed_note("recipes/lentil-soup.md", &parsed)
            .unwrap();

        let recipes = db.list_recipes(&[]).unwrap();
        assert_eq!(recipes.len(), 1);
        assert_eq!(recipes[0].note_id, note_id);
        assert_eq!(recipes[0].name, "lentil-soup");
        assert_eq!(recipes[0].serving_size, "4");
        assert_eq!(recipes[0].meal, "dinner");
        assert!(recipes[0].instructions.contains("Rinse lentils"));
        assert!(recipes[0].payload_json.contains("lentils"));

        let scoped = db.list_recipes(&["dinner".to_string()]).unwrap();
        assert_eq!(scoped.len(), 1);
        let missing = db.list_recipes(&["breakfast".to_string()]).unwrap();
        assert!(missing.is_empty());

        let ingredients = db.list_ingredients().unwrap();
        assert_eq!(ingredients.len(), 2);
        assert_eq!(ingredients[0].name, "lentils");
        assert_eq!(ingredients[0].mention_count, 1);

        let mentions = db.list_ingredient_mentions(false).unwrap();
        assert_eq!(mentions.len(), 2);
        assert_eq!(mentions[0].raw_name, "lentils");
        assert_eq!(mentions[0].quantity_number, Some(1.5));
        assert_eq!(mentions[0].unit_raw, "cups");
        assert_eq!(mentions[0].canonical_name, "lentils");
    }

    #[test]
    fn queries_graph_nodes_and_edges() {
        let temp = tempfile::tempdir().unwrap();
        let db_path = temp.path().join("notes.db");
        let mut db = NoteDb::open(&db_path, Some(EMBEDDED_SCHEMA_PATH)).unwrap();

        let hub = mw_notes::parse_note(
            r#"---
domains: [project]
tags: [seed]
---

See [[benefits]] and [[areas/current/child.md|Child]].
"#,
            "hub.md",
        );
        let benefits = mw_notes::parse_note("# Benefits\n", "benefits.md");
        let child = mw_notes::parse_note(
            r#"---
domains: [research]
---

# Child
"#,
            "areas/current/child.md",
        );
        let hub_id = db.upsert_parsed_note("hub.md", &hub).unwrap();
        let benefits_id = db.upsert_parsed_note("benefits.md", &benefits).unwrap();
        let child_id = db
            .upsert_parsed_note("areas/current/child.md", &child)
            .unwrap();
        db.replace_registry(
            &[
                RegisteredNoteId {
                    note_id: Some(hub_id),
                    uid: "hub".to_string(),
                    path: "hub.md".to_string(),
                    is_hub: true,
                },
                RegisteredNoteId {
                    note_id: Some(benefits_id),
                    uid: "benefits".to_string(),
                    path: "benefits.md".to_string(),
                    is_hub: false,
                },
                RegisteredNoteId {
                    note_id: Some(child_id),
                    uid: "child".to_string(),
                    path: "areas/current/child.md".to_string(),
                    is_hub: false,
                },
            ],
            &[],
        )
        .unwrap();

        let graph = db.query_graph("hub", "", 1, 250).unwrap();
        assert_eq!(graph.meta.seed_count, 1);
        assert_eq!(graph.nodes.len(), 3);
        assert_eq!(graph.edges.len(), 2);
        assert!(graph.nodes.iter().any(|n| n.uid == "hub" && n.matched));
        assert!(graph.edges.iter().any(|e| {
            e.source == format!("note:{hub_id}") && e.target == format!("note:{benefits_id}")
        }));
        assert!(graph.edges.iter().any(|e| {
            e.source == format!("note:{hub_id}") && e.target == format!("note:{child_id}")
        }));

        let research = db.query_graph("", "research", 0, 250).unwrap();
        assert_eq!(research.nodes.len(), 1);
        assert_eq!(research.nodes[0].uid, "child");
        assert!(research.edges.is_empty());
    }

    #[test]
    fn replaces_registry_entries_and_conflicts() {
        let temp = tempfile::tempdir().unwrap();
        let db_path = temp.path().join("notes.db");
        let mut db = NoteDb::open(&db_path, Some(EMBEDDED_SCHEMA_PATH)).unwrap();
        let parsed = mw_notes::parse_note("# Note", "note.md");
        let note_id = db.upsert_parsed_note("note.md", &parsed).unwrap();

        db.replace_registry(
            &[RegisteredNoteId {
                note_id: Some(note_id),
                uid: "note".to_string(),
                path: "note.md".to_string(),
                is_hub: false,
            }],
            &[RegistryConflict {
                note_id: None,
                uid: Some("dup".to_string()),
                path: "dup.md".to_string(),
                is_hub: false,
                reason: "DUPLICATE_ID".to_string(),
            }],
        )
        .unwrap();

        assert_eq!(db.count_registry_entries().unwrap(), 1);
        assert_eq!(db.count_registry_conflicts().unwrap(), 1);
        let entries = db.list_registry_entries().unwrap();
        assert_eq!(entries.len(), 1);
        assert_eq!(entries[0].uid, "note");
        assert_eq!(entries[0].path, "note.md");
        let conflicts = db.list_registry_conflicts().unwrap();
        assert_eq!(conflicts.len(), 1);
        assert_eq!(conflicts[0].path, "dup.md");
        assert_eq!(conflicts[0].reason, "DUPLICATE_ID");

        db.replace_registry(&[], &[]).unwrap();
        assert_eq!(db.count_registry_entries().unwrap(), 0);
        assert_eq!(db.count_registry_conflicts().unwrap(), 0);
    }

    #[test]
    fn queries_notes_by_id_uid_title_tags_and_domain() {
        let temp = tempfile::tempdir().unwrap();
        let db_path = temp.path().join("notes.db");
        let mut db = NoteDb::open(&db_path, Some(EMBEDDED_SCHEMA_PATH)).unwrap();
        let parsed = mw_notes::parse_note(
            r#"---
domains: [glossary]
tags: [alpha]
---

See [[target]].
"#,
            "biology/rna.md",
        );
        let note_id = db.upsert_parsed_note("biology/rna.md", &parsed).unwrap();
        db.replace_registry(
            &[RegisteredNoteId {
                note_id: Some(note_id),
                uid: "rna".to_string(),
                path: "biology/rna.md".to_string(),
                is_hub: false,
            }],
            &[],
        )
        .unwrap();

        assert_eq!(db.list_notes(50, 0).unwrap().len(), 1);
        assert_eq!(
            db.get_note_by_id(note_id).unwrap().unwrap().path,
            "biology/rna.md"
        );
        assert_eq!(db.get_note_by_uid("rna").unwrap().unwrap().uid, "rna");
        assert_eq!(db.search_notes_by_title("rna").unwrap().len(), 1);
        assert_eq!(
            db.list_notes_by_tags(&["alpha".to_string()]).unwrap().len(),
            1
        );
        assert_eq!(db.list_notes_by_domain("glossary").unwrap().len(), 1);
        assert_eq!(db.list_domains().unwrap(), vec!["glossary"]);
    }
}
