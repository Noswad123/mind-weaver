use std::{fs, path::Path};

use anyhow::{Context, Result, anyhow, bail};
use mw_core::{EMBEDDED_COMMAND_SCHEMA_PATH, EMBEDDED_SCHEMA_PATH, config::Config};
use mw_notes::{LinkType, ParsedNote};
use rusqlite::{Connection, params};

const EMBEDDED_NOTE_SCHEMA: &str = include_str!("../schema.sql");
const EMBEDDED_COMMAND_SCHEMA: &str = include_str!("../command-schema.sql");

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
        self.conn
            .execute("DELETE FROM notes WHERE path = ?1", [path.trim()])?;
        Ok(())
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

fn clear_note_children(tx: &rusqlite::Transaction<'_>, note_id: i64) -> Result<()> {
    tx.execute("DELETE FROM note_domains WHERE note_id = ?1", [note_id])?;
    tx.execute("DELETE FROM tags WHERE note_id = ?1", [note_id])?;
    tx.execute("DELETE FROM links WHERE note_id = ?1", [note_id])?;
    tx.execute("DELETE FROM todos WHERE note_id = ?1", [note_id])?;
    tx.execute("DELETE FROM task_groups WHERE note_id = ?1", [note_id])?;
    Ok(())
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
